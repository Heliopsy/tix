# Migrations see every tenant's rows on PostgreSQL

## Why

On PostgreSQL a migration ran as the login role with no `tix.tenant_id` set, under forced row-level
security. On a role without BYPASSRLS, which is what production runs, a data step saw no rows and
reported success. `actor_user_link` (0.46.0) and `actor_display_name_from_account` (0.48.0) both did
nothing there, so every account created before 0.46.0 kept an unlinked actor: it could not sign in, was
missing from the user list and lost its global admin standing.

## What Changes

- Every migration transaction sets `tix.migrating` with `set_config(..., true)`, so it ends with the
  transaction. Only the migration runner sets it.
- Every forced table gets a permissive `tix_migrating` policy that admits that flag for every command,
  and refuses `tix_app`, the role runtime transactions enter.
- A new migration, `repair_actor_links`, runs both backfills again. It is a no-op where they already
  worked (SQLite) or where the link was repaired by hand.

## Capabilities

### Modified Capabilities

- `storage-engines`: a migration's data steps reach every tenant's rows on PostgreSQL.

## Impact

`internal/store/postgres/schema.go`, a migration under `internal/store/migrations`,
`internal/store/postgres/README.md`, `docs/migrating.md`.
