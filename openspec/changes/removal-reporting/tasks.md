# Tasks

## Project deletion counts

- [x] `core.ProjectDeletion`: `trashed`, `deleted` as the removed count, `would_delete` on a plan
- [x] `Local.DeleteProject` and `Local.PlanProjectDeletion` fill them
- [x] Table output reads the new counts; web confirmation reads `Trashed`
- [x] Service test, CLI test on SQLite and PostgreSQL

## Removed people

- [x] `core.UserFilter`, `core.User.Removed`; `ListUsers` and `ListTenantUsers` take the filter
- [x] `Local.ListUsers` leaves out actors with no membership unless asked; `GetUser` marks them
- [x] HTTP `include_removed` (`httpapi.UserListQuery`), client, registry contracts
- [x] `--include-removed` on `tix user ls` and `tix tenant user ls`; tables mark removed people
- [x] Web Users and global users screens, TUI people and global admin views ask for current people only
- [x] Activity screen still resolves removed people's names
- [x] Pending invites of another tenant's person stay out of the listing
- [x] Service tests (one tenant and two), web test, CLI test on SQLite and PostgreSQL
- [x] `just openapi`, docs (api, scripting, tenancy) and the tix skill
