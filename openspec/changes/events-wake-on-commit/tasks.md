# Tasks

## 1. The tailer selects on a wake beside its ticker

- [x] 1.1 `internal/outbox/outbox.go`: `Tailer.wake`, `WithWake`, and a third case in the idle select.
      The doc comment states that the ticker is not replaced and why: a dropped signal must cost
      latency and never an event
- [x] 1.2 `internal/outbox/outbox_test.go`: `TestWakeDeliversWithoutWaitingForTheTicker` with a 30s
      poll, `TestTheTickerStillDeliversWhenNoWakeArrives` with a wake that never fires, and
      `TestATailerWithoutAWakeChannelStillDelivers` for the nil channel

## 2. The signal an engine can offer

- [x] 2.1 `internal/store/wake.go`: `CommitWaker`, the optional interface, and `Wakers`, the per-tenant
      fan-out both engines share. The send is non-blocking on a buffer of one, so a full buffer never
      stalls a committing transaction
- [x] 2.2 `internal/store/wake_test.go`: a wake reaches only its tenant, repeated wakes coalesce and
      never block, and a receiver is forgotten when its context ends

## 3. PostgreSQL stops ignoring the notification it already sends

- [x] 3.1 `internal/store/postgres/listen.go`: `WakeOnCommit` and the fan-out goroutine over the
      existing `Listen`, one listener connection per process, retaken a second after it fails
- [x] 3.2 `internal/store/postgres/postgres.go`: `Store.wakers` and the listener cancel, stopped by
      `Close` before the pool goes

## 4. SQLite wakes in process

- [x] 4.1 `internal/store/sqlite/event.go`: `AppendEvent` arms `tx.notify`, as the PostgreSQL
      transaction does
- [x] 4.2 `internal/store/sqlite/tx.go`: `Commit` signals the fan-out after `COMMIT` returns and the
      writer connection is released, so a woken reader never reads a position that is not there yet
- [x] 4.3 `internal/store/sqlite/sqlite.go`: `WakeOnCommit`, whose doc comment states that a separate
      process writing the same file is seen by the ticker and not by this signal
- [x] 4.4 `internal/store/sqlite/wake_test.go`: a commit wakes a reader, a transaction with no events
      does not, and a wake does not cross tenants

## 5. The two consumers

- [x] 5.1 `internal/server/events.go`: `pumps.wake` asks the store for a channel and passes
      `outbox.WithWake`; a store that is not a `CommitWaker`, or that refuses, leaves the reader on its
      ticker and logs it
- [x] 5.2 `internal/service/history.go`: the direct database path does the same, which is what
      `tix watch` and the terminal board use against a database file
- [x] 5.3 `internal/server/assemble.go`: `EventPollInterval` is documented as the floor under a reader
      rather than as how it learns about events
- [x] 5.4 `internal/service/history_test.go`:
      `TestACommitReachesADirectSubscriberWithoutWaitingForThePoll` measures ten commits end to end
      against a one second budget, with the comment saying what the budget protects

## 6. Gates

- [x] 6.1 Measured before and after on the same test: 2.25s for ten commits on the ticker, 1.36ms to
      3.10ms over five runs after, around 0.23ms per event
- [x] 6.2 Watched the guard fail: the wake case removed from the tailer's select, the SQLite
      `Commit` signal removed, and `AppendEvent`'s flag removed. Restored with `git checkout --`,
      `git diff` empty after each
- [x] 6.3 `just check` exit 0. The security and vulnerability gates in `just ci` were not run here
