# A commit wakes the event stream

## Why

tix claims to be faster to work with than Jira. The event delivery path does not
support that claim, and the reason is a clock.

`outbox.Tailer.run` reads events since its cursor, dispatches them, and when the
batch was short, blocks on a ticker. The ticker defaults to 250ms. So every
write waits an average of 125ms, and up to 250ms, before any subscriber hears
about it: a board refresh, a `tix watch` line, a WebSocket frame. It waits that
long even when the writer and the reader are the same process, which is the
single-binary deployment almost everyone runs, including `tix serve
--ssh-listen` where an SSH session and the web server share a process.

Measured on the direct database path: ten commits took **2.25s** to reach a
subscriber, 225ms each.

PostgreSQL pays the same 250ms for nothing. `internal/store/postgres/tx.go`
already issues `SELECT pg_notify` on a commit that appended events, and
`internal/store/postgres/listen.go` already exposes `Store.Listen`, tested end
to end. **Nothing outside the postgres package calls it.** The notification path
is built, works, and has no subscriber.

## What Changes

**The tailer wakes on a commit. The ticker stays, demoted to a safety net.**

- **`outbox.WithWake`** gives a tailer a channel it selects on beside its
  ticker. The ticker is kept at 250ms and is not weakened, because correctness
  must not depend on a signal arriving. A wake that is dropped, coalesced or
  never wired costs latency until the next tick and loses no event. That is the
  property the poll was chosen for and it is the one thing this change must not
  spend.

- **`store.CommitWaker`** is the optional interface an engine implements to
  announce a commit that appended events. `store.Wakers` is the shared fan-out
  behind it: both engines differ in where the signal comes from, not in what is
  done with it. A store that does not implement it is polled exactly as before.

- **PostgreSQL finally listens.** `Store.WakeOnCommit` starts one `LISTEN`
  connection for the whole process on first use and fans its notifications out
  by tenant. A dropped listener is retaken after a second; readers poll
  meanwhile.

- **SQLite wakes in process, with no database round trip.** The SQLite
  transaction arms a flag when it appends an event and signals the store's
  fan-out after `COMMIT` returns.

- **Cross-process SQLite is documented, not hidden.** A separate `tix`
  invocation writing the same file cannot be heard: SQLite has no cross-process
  notification and inventing one would mean a channel outside the database.
  Those writes are seen by the ticker, late rather than never, and both the
  `WakeOnCommit` doc comment and the spec say so.

- **Two consumers are wired:** the server's per-tenant pumps
  (`internal/server/events.go`) and the direct database path
  (`internal/service/history.go`), which is what `tix watch` and the terminal
  board use against a database file.

## Impact

- `internal/outbox/outbox.go`: `WithWake`, and a third case in the idle select.
- `internal/store/wake.go`: new. `CommitWaker` and `Wakers`.
- `internal/store/sqlite/`: `Store.wakers`, `tx.notify`, `WakeOnCommit`.
- `internal/store/postgres/`: `Store.wakers`, the listener fan-out, `WakeOnCommit`,
  and `Close` stopping the listener.
- `internal/server/events.go`, `internal/service/history.go`: the two consumers.
- `internal/server/assemble.go`: `EventPollInterval` is documented as the floor,
  not the mechanism.

Measured after the change on the same path, five runs: ten commits in 1.36ms to
3.10ms, **2.3ms** typical, around 0.23ms per event. Roughly a thousandfold
improvement, and what is left is the commit itself rather than a wait.

**What breaks.** Nothing. No wire protocol change, no event schema change, no
signature change on `core.Service`. The outbox keeps ordering by `seq`, gap-free
`since_seq` resume and its retry and backoff behaviour on a failing read: the
wake only shortens an idle wait, and a tailer handed no wake behaves exactly as
it does today. `CGO_ENABLED=0` is unaffected; no dependency was added.
