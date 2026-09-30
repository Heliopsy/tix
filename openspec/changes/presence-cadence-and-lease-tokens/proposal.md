# Staleness follows the server's own cadence, a vanished row comes back, and a stale token is refused

## Why

Three defects, two in what `tix status` reports about running servers and one in what a write under a
lease is allowed to do.

**`--heartbeat-interval` moved the writing and nothing moved the reading.** The staleness threshold was a
compile-time constant, three times the default 30-second cadence, and `Server.Attached` read it. The flag
changed only how often the row was refreshed, and there was no column to record what it had been changed
to, so a reader judged every server against a cadence it might not keep. That is wrong in both directions
and both are real: a live server beating every five minutes reads as down two and a half minutes after its
last beat, and a dead server that had been beating every second still reads as up for ninety. Every
consumer of `tix status`, `StatusReport.AttachedCount` and the browser status screen inherits it.

**A running server whose row disappeared never came back.** `HeartbeatServer` discarded the row count from
its `UPDATE` and returned success, so the registrar could not tell a refreshed row from one that no longer
existed. `Run` re-registers only while its local `registered` flag is false, and that flag never became
false again once the first registration landed, so a process whose row was deleted underneath it -- by an
operator, by another server's purge, by a database restored from a backup taken before it started -- beat
into the void for the rest of its life. This is the failure `Run` already carries a comment about:
under-reporting what is running reads as "that server is down". The retry path was written for a
registration that never landed; the symmetric case was uncovered.

**A lease that ran out still let its old holder write.** Two near-duplicate guards existed. `checkLease`,
the one production used, returned success when a token was offered for a task that was no longer claimed.
`requireLeaseToken`, which no caller anywhere reached, refused it. So a worker whose lease lapsed could go
on transitioning a task that was free again and may already have been claimed and released by somebody
else, and the only test asserting the stricter behaviour was asserting it of a function nothing called.

## What Changes

- **A server records the cadence it promised to beat at**, in a new `servers.heartbeat_interval_ms`
  column, and a reader derives that server's threshold from its own row: still three missed beats, but
  three of the beats that row declares. A row that declares none -- every row written before this change
  -- is judged against the default interval, which is exactly what it was judged against before.
- **`heartbeat_interval` joins the status document**, beside `last_seen_at`, so a consumer that wants a
  finer judgement than the `attached` boolean has the threshold rather than having to assume one.
- **`HeartbeatServer` reports a beat that matched no row** as a not-found rather than as success, and
  `Registrar.Run` answers that by registering again on the next tick.
- **One lease guard, and it refuses a token offered for a lease that is no longer live.**
  `checkLease` is deleted, `TransitionTask` calls `requireLeaseToken`, and the guard test drives
  `TransitionTask` rather than calling the guard directly. A caller carrying no token is unaffected: a
  task whose lease has expired reads as unclaimed and needs none, which is what makes lazy expiry
  authoritative.

Lease tokens are deliberately **not** extended to any further write path here. Whether `UpdateTask`,
comments and artifacts should carry one is a separate question.

## Impact

- Affected specs: `server-status`, `claim-lease`
- Affected code: `internal/core/status.go`, `internal/presence/registrar.go`,
  `internal/store/sqlite/server.go`, `internal/store/postgres/server.go`, `internal/service/claim.go`,
  `internal/service/task.go`, `docs/scripting.md`
- Schema change: migration `0009_server_heartbeat_interval.sql`, one additive column with a default, so an
  upgraded database and a fresh one hold the same table.
- Behaviour change on the wire: `ServerStatus` gains `heartbeat_interval`, and `TransitionTask` now
  refuses a lease token offered for a task that is not claimed.
