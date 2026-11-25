# Removals report what they did

## Why

Two things read wrong on a production server after a clean-up.

`tix project rm KEY --delete-tasks` deleted 868 tasks and printed `"deleted": 0`. The field counted the
project's tasks already in the trash, not the tasks the deletion removed, so the dry run and the real run
printed the same zero and neither said how many tasks went.

`tix user rm ID` removed two people from a tenant, and `tix user ls` kept listing them, with no role. Their
actor stays in the tenant so history still names them, which is right, but the users listing treated every
actor with an account as a current user. The web Users screen, `ListUsers` over HTTP, the TUI people view
and the global admin's `tenant user ls` all had the same gap.

## What Changes

- `ProjectDeletion.deleted` counts the tasks the deletion removed permanently, trashed ones included. A plan
  carries `would_delete` instead and keeps `deleted` at zero. The count of trashed tasks moves to `trashed`.
- User listings (`ListUsers`, `ListTenantUsers`) leave out people whose membership of the tenant is gone.
  `include_removed` (HTTP) and `--include-removed` (CLI) list them too, each marked `removed`; the global
  admin's table shows the role `removed`. `GetUser` marks a removed person the same way.
- The web Users screens and the TUI people view leave removed people out. The activity screen still names
  them, since history is what their actor is kept for.

## Capabilities

### Modified Capabilities

- `cli`: the deletion result's counts, `--include-removed` on `user ls` and `tenant user ls`.
- `http-api`: `include_removed` on the two user listing routes.
- `multi-tenancy`: a removed person is not a current user of the tenant.
- `web-ui`: the Users screens leave removed people out.
- `tui`: the people view leaves removed people out.

## Impact

`internal/core` (`ProjectDeletion`, `User.Removed`, `UserFilter`, the two listing signatures),
`internal/service`, `internal/httpapi`, `internal/client`, `cmd`, `internal/output`, `internal/web`,
`internal/tui`, `api/openapi.yaml`. A client reading `deleted` from a project deletion as the trash count
reads `trashed` now.
