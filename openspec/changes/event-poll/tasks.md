# Tasks

## Service

- [x] `core.EventPollInput`, `core.EventPage` and `PollEvents` on `core.HistoryService`
- [x] Event entitlement, type vocabulary and expired-cursor error moved to `internal/service`; WebSocket calls them
- [x] `Local.PollEvents`: validation, narrowing, retention check, bounded read, wait on the commit signal
- [x] Table-driven service tests on SQLite with `clock.FakeClock`

## Surfaces

- [x] Route `GET /api/v1/events/poll`, `httpapi.EventPollQuery`, handler and HTTP tests
- [x] `internal/client.PollEvents`
- [x] Registry entry `event.poll` with web and TUI exemptions
- [x] `tix events` and generated CLI docs
- [x] MCP overlay hides `wait`
- [x] `just openapi`, `docs/api.md` "Following events over HTTP", tix skill
