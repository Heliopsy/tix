# Design

## The window

Three things happen when a tenant's first subscriber arrives:

1. `EventStream.ServeHTTP` upgrades the socket and calls `Hub.Register`.
2. `Hub.Register` crosses the tenant's connection count from zero and fires the first-connection hook, which
   is `pumps.start`. The hook runs under `hookMu`, the lock that keeps hook order matching count order.
3. The connection's read loop later handles `subscribe`, replays the durable log from the client's cursor,
   and goes live.

Replay covers `(client cursor, latest at replay time]`. The live stream covers `(pump cursor, ...)`. The two
are gap-free only if the pump cursor is no higher than the latest sequence at the moment step 2 completed.
Taking it inside the spawned goroutine makes that moment unbounded, and an event committed in between is
covered by neither range.

## Decision: take the cursor before `start` returns

`pumps.start` builds the tenant's reader and reads `Latest` itself, then hands both to the goroutine, which
subscribes with `EventFilter{SinceSeq: cursor}`. The cursor is now pinned before the hub can acknowledge or
replay anything, so the two ranges meet by construction rather than by scheduling luck.

A cursor read that fails releases the tenant's slot and logs, so the next connection starts a fresh pump
instead of finding a stale entry and going silent.

## Accepted cost: connects serialise on the cursor read

`Hub.Register` holds `hookMu` across the hook, so the `SELECT MAX(seq)` now runs inside that lock. Every
connect and every disconnect in the process, across all tenants, waits for it. The query is indexed and
single-row, but it is still a database round trip on a lock that previously held only bookkeeping.

This is accepted deliberately. Connection churn is low relative to event volume, and losing an event is a
correctness failure while a slower connect is a throughput one.

The cleaner fix, deliberately not taken here, is to stop acknowledging `subscribed` before the tenant's live
cursor exists: the connection would wait on the per-connection subscribe path, off the global lock
altogether. That spans the hub, the event stream and the pump, and wants its own change.

## Guard

`TestPumpTakesItsLiveCursorBeforeStartReturns` asserts the property, not the placement of a call. It runs a
real hub, a real pump over a temporary SQLite store and a real WebSocket subscriber, holds the tenant
reader's cursor read to stand in for a goroutine the scheduler has not run, commits an event once the
connection is registered, and requires that event to reach the client. A test that asserted which goroutine
called `Latest` would pass a refactor that reopened the gap by another route.

## Correction carried by this change

The tix-v1 design note said the SQLite tailer wakes on `PRAGMA data_version` polling. That was never built:
`internal/outbox` re-runs a `seq > ?` query on a ticker, and only the PostgreSQL half uses `LISTEN/NOTIFY`.
The note is corrected in place, because it describes implemented mechanism rather than a decision of record,
and reading it as current truth sent this investigation looking for a mechanism that does not exist.
