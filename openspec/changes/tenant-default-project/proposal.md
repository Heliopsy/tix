## Why

A tenant made by a global admin starts with no project, so its first screen is an empty sidebar and the
first project someone adds by hand has no icon or colour. A fresh installation already opens on a
Default list with an icon and a colour; a new tenant should open the same way.

## What Changes

- Creating a tenant also creates its Default project, with the same icon and colour the starter Default
  list has on a fresh installation.
- Seeding stays idempotent: a tenant that already has a project keyed `default` is left alone.

## Impact

- `internal/service/tenant.go`, `internal/service/bootstrap.go`
- Existing tenants are unchanged.
