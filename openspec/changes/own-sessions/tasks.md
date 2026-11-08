# Tasks

## Service

- [ ] `core.SessionInfo`, `ListSessions` and `RevokeSession` on `core.AuthService`
- [ ] `authz.Policy.CanManageSessions`
- [ ] Store: list and delete an actor's session by identifier, both engines; user agent on create
- [ ] Service methods with audit and event in one transaction, tests on SQLite with `clock.FakeClock`

## Surfaces

- [ ] Routes `GET /api/v1/account/sessions`, `DELETE /api/v1/account/sessions/{id}`, handlers and client
- [ ] Registry entries `session.list` and `session.revoke`
- [ ] `tix account sessions`, `tix account sessions revoke ID`, generated CLI docs
- [ ] Web settings Sessions section with confirm dialog, Bulgarian strings
- [ ] Terminal account view lists and revokes sessions
- [ ] MCP overlay hides both
- [ ] `just openapi`, docs, tix skill
