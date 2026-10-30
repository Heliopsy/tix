# Design

## The property being delivered

A committed event reaches a subscriber in the time it takes to send on a
channel, and never later than the poll interval. Two numbers, not one: the
first is what a person feels, the second is what the system guarantees. The
whole design follows from keeping them separate.

## Why the ticker stays, and stays at 250ms

The obvious version of this change replaces the poll with a notification. It is
wrong, and it is worth recording why, because every argument for it is correct
except the one that matters.

A poll is a design that cannot lose an event. It asks the durable log what is
above its cursor; if the answer is nothing, it asks again later. Nothing in the
path can drop, reorder or coalesce anything, because nothing in the path carries
information. A notification carries information, and therefore can be lost: a
`LISTEN` connection drops between the `pg_notify` and the reader, a process
restarts in the gap, a buffered channel is full, a signal arrives for a tenant
whose reader has not registered yet. Each of those is rare. Each of those, in a
notification-only design, is a silently missing event, and a silently missing
event in a task queue is an agent that never starts work.

So the wake is an optimisation on top of the poll and is never load-bearing.
`outbox.Tailer.run` selects on the wake, the ticker and the context together; a
nil wake channel blocks forever and the tailer behaves exactly as it did. The
comment on `WithWake` states this, because the next person to read the select
will see a ticker that appears redundant and that appearance is the hazard.

The interval is left at 250ms rather than lengthened. Lengthening it is the
tempting second half of the change: the ticker rarely fires now, so why not make
it 5s and save the wakeups. Because the interval is the worst case for every
path the wake does not cover, and there is a real one — a second process writing
the same SQLite file, below. 250ms is the latency that shipped, nobody has
complained about it as a floor, and the saving from raising it is a few timer
firings per idle tenant. Raising it would trade a guarantee for nothing
measurable.

## One interface, two signals

`store.CommitWaker` is an optional interface, asserted for rather than added to
`store.Store`:

```go
WakeOnCommit(ctx context.Context, tenantID string) (<-chan struct{}, error)
```

Optional because a store that cannot wake must stay usable, and because the
assertion is the honest expression of "some engines can do this". A consumer
that gets no channel passes no `WithWake` and polls.

The two engines differ only in where the signal comes from, so the fan-out is
shared: `store.Wakers` keys receivers by tenant, signals with a non-blocking
send on a buffer of one, and releases a receiver when its context ends. The
coalescing is deliberate. A receiver that has not yet drained the first signal
has not yet read the position that signal pointed at, so a second one tells it
nothing; and the send must never block, because the thing blocked would be a
committing transaction holding SQLite's single writer connection.

### PostgreSQL: the path that already existed

`tx.Commit` has issued `SELECT pg_notify` on an event-bearing transaction since
the engine was written, and `Store.Listen` has consumed it, with tests. Nothing
called `Listen`. This change calls it.

One listener serves the whole process. The alternative, a `Listen` per tailer,
is simpler and was rejected: `Listen` takes a dedicated connection out of the
pool and holds it, so a server tailing twenty tenants would hold twenty
connections doing nothing but waiting. The fan-out starts on the first
`WakeOnCommit`, is stopped by `Store.Close`, and retakes its connection a second
after it fails. A failure there is a gap in wake-ups and not a gap in the log,
so it is ridden out rather than reported upwards: the readers are already
polling and already correct.

### SQLite: in process, deliberately

A SQLite commit signals the store's fan-out directly, after `COMMIT` returns and
the writer connection is released. No database round trip, no file, no socket.
This covers the deployment nearly everyone runs: one binary serving HTTP, the
browser, the WebSocket stream and, under `tix serve --ssh-listen`, the terminal
interface, all in one process, where the writer and the reader are objects in
the same heap.

The flag that arms it, `tx.notify`, is set by `AppendEvent` and read by
`Commit`, mirroring the PostgreSQL transaction exactly. A transaction that wrote
no events announces nothing, so a reader is not woken to find its cursor
unchanged.

## What SQLite cannot do, said out loud

A second process writing the same SQLite file cannot be heard. `tix task add`
run from a terminal against the file a running `tix serve` has open commits, and
the server's in-process fan-out knows nothing about it. The server sees that
event on its next tick, so within 250ms, exactly as it did before this change.

This is not a defect to be fixed later behind a TODO, and three ways of fixing
it were considered and declined:

- **SQLite's own update hooks** are a C callback on the connection that made the
  change. They do not cross processes, and reaching them at all would mean cgo,
  which `AGENTS.md` forbids and which would end cross-compilation.
- **A file watch** on the database or its WAL turns a storage detail into a
  notification channel, fires on checkpoints and vacuum as well as commits,
  behaves differently on every platform, and does not work on the network
  filesystems the store already refuses for other reasons.
- **A side channel**, a Unix socket or a lock file poked on commit, is a second
  coordination mechanism with its own staleness and permission problems, built
  so that a deployment with more than one writer can avoid a 250ms wait. The
  deployment that genuinely has many writers is the one that should be on
  PostgreSQL, where the notification exists already and crosses processes
  properly.

So the honest statement is that the SQLite wake covers one process, the ticker
covers the rest, and the cost of the uncovered case is the latency tix shipped
with. It is written in the `WakeOnCommit` doc comment, in the proposal and in
the spec delta, rather than left for someone to discover by measuring.

## Measuring the property, not the mechanism

Three tests, at three levels, because one of them alone would be either flaky or
vacuous.

`internal/outbox` proves the mechanism with no timing judgement at all: a tailer
polling every 30 seconds delivers within 2 seconds of a wake. Anything that
arrives came from the wake, and 2 seconds is far beyond what passing a value
down a channel needs on any machine. Its sibling proves the safety net, with a
wake channel that never fires and a 1ms ticker, because a change that broke the
fallback would otherwise pass every other test here.

`internal/store/sqlite` proves the engine signals on commit, that a transaction
carrying no events does not, and that a wake does not cross tenants.

`internal/service` measures the end-to-end property a person experiences:
`TestACommitReachesADirectSubscriberWithoutWaitingForThePoll` commits ten events
and waits for each. The budget is one second for all ten. That number is chosen
to be undiscriminating about speed and very discriminating about mechanism: on
the ticker alone this takes about 2.25 seconds, measured, and it is not plausible
for ten ticker waits to fit in one second; with the wake it takes 2.3ms, so the
budget leaves more than two orders of magnitude of slack per round trip for a
loaded CI machine. A single-event bound of 100ms would have been the obvious
test and would have passed a broken wake four times in ten, because a lucky
commit lands just before a tick.

`clock.FakeClock` is not used here. The thing under test is a real `time.Ticker`
losing a race to a real channel send; a fake clock would decide the race by
construction and prove nothing about it.
