# Tenant and theme on whoami for every credential

## Why

Building tix-mobile turned up two gaps in `GET /whoami`.

- A 0.48 server answered an API token with `tenant_id` alone, while docs/tenancy.md says whoami reports
  `tenant_key` and `tenant_name`. 0.49 fills them for a token, but still only for a user actor or a token:
  any other credential gets nothing, because the tenant lookup sits behind the account check.
- A client cannot paint the tenant's accent. The tenant record, theme name included, is readable only
  with `tenant:admin`, and themes defined in configuration never reach anything: the `themes` block is
  refused as an unknown key when configuration loads, its dark pair has no field to land in, and the
  service is built without the configured registry, so `tix tenant edit --theme NAME` would refuse a
  configured name even if it loaded.

## What Changes

- whoami reports `tenant_key` and `tenant_name` for every credential. Memberships keep their current rule:
  a session, or a token carrying `account:read`.
- whoami carries `theme`, the palette the browser renders for the tenant: `name`, `accent`, `accent_soft`,
  `accent_dark`, `accent_soft_dark` and `built_in`, with the dark pair always filled. No other tenant setting
  is exposed.
- Configuration reads the `themes` block, dark pair included, from the global and the per-directory file
  (the latter wins by name), and keeps it when a command rewrites the file.
- The service is built with the configured theme registry, so configured themes resolve and can be named
  from the CLI and the API, not only in the browser.
- `tix whoami` prints the theme.

## Capabilities

### Modified Capabilities

- `http-api`: whoami names the tenant for every credential and carries its resolved theme.
- `theming`: the resolved theme is readable by any member, and configured themes load and resolve in the
  service.

## Impact

`internal/core` (`Actor.Theme`), `internal/service` (describeAccount), `internal/config` (themes block),
`internal/connect` (registry wiring), `cmd` (whoami output), `api/openapi.yaml`, docs/tenancy.md, docs/theming.md, docs/api.md,
the tix skill.
