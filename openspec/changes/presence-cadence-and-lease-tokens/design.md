# Design

## Where the staleness threshold has to live

A reader judges a server by comparing its own clock to the row's `last_seen_at`. That judgement needs a
threshold, and the threshold is a property of the writer: only the process doing the beating knows how
often it beats. Three ways to supply it were available.

| Option | Why not |
| --- | --- |
| Keep the constant, document the flag as advanced | The flag exists and is documented in `docs/deployment.md`. A flag that silently breaks the report it feeds is worse than no flag. |
| Make the reader take the interval as a parameter | The reader is `tix status` run from anywhere, against servers it has never heard of and which may each beat at a different rate. There is no one value to pass. |
| **Record the cadence on the row** | The writer is the only party that knows it, and it is already writing the row. |

So: `servers.heartbeat_interval_ms`, an integer count of milliseconds, written by `RegisterServer` from
`Registrar.interval`. `Server.StaleAfter()` returns three of it; `Server.Attached` calls that.

Milliseconds as an integer rather than an interval type, because an interval is a count rather than an
instant and neither engine's native interval type is worth a second scanner per engine for a value this
shape. `core.Duration` already round-trips to a readable string in JSON and YAML, so the field the reader
sees is `"30s"` rather than `30000`.

### What an old row means

Zero: the row was written by a server that did not say. A reader resolves that to
`core.ServerHeartbeatInterval`, which is what every such row was already being judged against, so the
migration changes nothing about a database that has not yet restarted its servers. The first beat an
upgraded server writes replaces the zero with the truth. `NOT NULL DEFAULT 0` rather than a nullable
column, because "did not say" and "said zero" are the same statement here and one of them is cheaper to
scan.

`ServerStaleAfter` stays, as the threshold for a server beating at the default interval, because that is
what the tests that name it mean and what an operator reading the constant expects. `ServerStaleBeats`
is the three.

## A beat that matches nothing

`execUpdate` already returns the row count on both engines; both `HeartbeatServer` implementations threw
it away. Returning `core.NotFound` for a zero count makes the store say the one thing the registrar needs
to hear, and `Run` treats it exactly as it already treats a registration that has not landed: clear
`registered`, and let the next tick register again.

Re-registering rather than re-inserting the old row is deliberate. `RegisterServer` replaces the row and
takes the current instant as `started_at` only when the registrar has none; the registrar fixed its own
`StartedAt` on its first attempt and keeps it, so the restored row reports the uptime the process actually
has. The identifier is the registrar's, so a reader sees the same server return rather than a new one
appear.

One tick of delay before the row returns is accepted. The alternative -- registering inline from the
failed beat -- doubles the work done inside one tick for a case that is rare, and the loop's shape is
easier to hold in mind with exactly one place that registers.

## Which lease semantics are right

The two implementations disagreed about one case: a token supplied for a task that is not currently
claimed.

`checkLease` returned `nil`. That means a worker whose lease lapsed keeps writing. Lazy expiry makes the
task free the instant the expiry passes, so between then and the worker noticing, a second worker may
claim it, work it and release it -- and the first worker's transition lands on top, attributed to nobody
in particular, with no lease to conflict against. The lease token exists precisely so that a holder learns
it is no longer the holder, and returning success is the one answer that never tells it.

`requireLeaseToken` returned a lease expiry, which `staleLease` words as "claim it again". That is the
action a worker can actually take, and it is what `ReleaseLease` and `RenewLease` already answer in the
same situation. So `requireLeaseToken`'s semantics are the correct ones and `checkLease` was the bug.

Only the error *kind* is taken from `checkLease`: a claimed task addressed with no token is a lease
expiry rather than a validation error, which is both what production already returned on that path and
the more coherent reading. All three refusals here state the same fact -- this caller does not hold this
lease -- so all three are one kind.

A caller carrying no token against an expired lease still proceeds. That is not an oversight: it is what
makes lazy expiry authoritative without a sweeper, and both implementations always agreed on it.

## Scope held

No other write path gains a lease token. `UpdateTask`, comments and artifacts are untouched here.
