# Tasks

## 1. Schema

- [x] 1.1 `internal/store/migrations/0009_server_heartbeat_interval.sql`: `servers.heartbeat_interval_ms`,
      `NOT NULL DEFAULT 0`, one additive statement so an upgraded table matches a fresh one
- [x] 1.2 `internal/store/migrations/migrations_test.go`: an upgraded database and a fresh one hold the
      same `servers` shape, and the pre-existing row keeps every value it had and declares no cadence

## 2. Core

- [x] 2.1 `internal/core/status.go`: `Server.HeartbeatInterval`, `ServerStaleBeats`, and
      `Server.StaleAfter()` derived per row with the default standing in for a zero
- [x] 2.2 `internal/core/status.go`: `Attached` judges against `StaleAfter()`, and `ServerStatus` carries
      the resolved `heartbeat_interval`

## 3. Store

- [x] 3.1 both engines: `serverColumns`, `scanServer` and `RegisterServer` carry the cadence
- [x] 3.2 both engines: `HeartbeatServer` returns `core.NotFound` when it matched no row
- [x] 3.3 both engines' `server_test.go`: the cadence round-trips and a beat against a deleted row is a
      not-found

## 4. Presence

- [x] 4.1 `internal/presence/registrar.go`: `Register` records `r.interval` on the row
- [x] 4.2 `internal/presence/registrar.go`: `Run` clears `registered` on a not-found beat, so the next
      tick registers again
- [x] 4.3 `internal/presence/registrar_test.go`: staleness in both directions against a declared cadence,
      the cadence reaching the row, and a deleted row returning under a running process

## 5. Lease guard

- [x] 5.1 `internal/service/task.go`: `checkLease` deleted, `TransitionTask` calls `requireLeaseToken`
- [x] 5.2 `internal/service/claim.go`: `requireLeaseToken` refuses a token offered against a lapsed lease,
      and reports every refusal on the path as lease expiry
- [x] 5.3 `internal/service/claim_test.go`: `TestLeaseTokenGuard` drives `TransitionTask` rather than the
      guard function, and asserts the task did not move on every refusal

## 6. Documentation

- [x] 6.1 `docs/scripting.md`: `heartbeat_interval` in the sample document and in the note about deriving
      a finer judgement than `attached`
