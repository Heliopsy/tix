# Tasks

## Service

- [ ] `core.EventPollInput`, `core.EventPage` and `PollEvents` on `core.HistoryService`
- [ ] Event entitlement, type vocabulary and expired-cursor error moved to `internal/service`; WebSocket calls them
- [ ] `Local.PollEvents`: validation, narrowing, retention check, bounded read, wait on the commit signal
- [ ] Table-driven service tests on SQLite with `clock.FakeClock`

## Surfaces

- [ ] Route `GET /api/v1/events/poll`, `httpapi.EventPollQuery`, handler and HTTP tests
- [ ] `internal/client.PollEvents`
- [ ] Registry entry `event.poll` with web and TUI exemptions
- [ ] `tix events` and generated CLI docs
- [ ] MCP overlay hides `wait`
- [ ] `just openapi`, `docs/api.md` "Following events over HTTP", tix skill
