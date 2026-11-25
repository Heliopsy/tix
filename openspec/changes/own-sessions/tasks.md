# Tasks

## Service

- [x] `core.SessionInfo`, `ListSessions` and `RevokeSession` on `core.AuthService`
- [x] `authz.Policy.CanManageSessions`, sharing `CanReadAccount`'s rule
- [x] Store: list and delete an actor's session by identifier, both engines; user agent on create
- [x] Service methods with audit and event in one transaction, tests on SQLite with `clock.FakeClock`

## Surfaces

- [x] Routes `GET /api/v1/account/sessions`, `DELETE /api/v1/account/sessions/{id}`, handlers and client
- [x] Registry entries `session.list` and `session.revoke`
- [x] `tix account sessions`, `tix account sessions revoke ID`, generated CLI docs
- [x] Web settings Sessions section with confirm dialog, Bulgarian strings
- [x] Terminal account view lists and revokes sessions
- [x] MCP overlay hides both
- [x] `just openapi`, docs, tix skill
