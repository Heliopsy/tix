# Tasks

## 1. Spec

- [x] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Storage

- [x] 2.1 Migration adding the icon columns and the image bytes to `tenants` (both engines)
- [x] 2.2 Tenant rows and memberships read and write the icon; `PutTenantIcon`, `GetTenantIcon`, `DeleteTenantIcon` on SQLite and PostgreSQL

## 3. Contract and service

- [x] 3.1 `core.Tenant`, `AccountMembership` and `Actor` icon fields, `UpdateTenantInput` icon fields and checks, `core.Monogram`
- [x] 3.2 `UpdateTenant` sets exactly one icon kind and writes the image in the same transaction
- [x] 3.3 `Service.GetTenantIcon`, readable by any member of the tenant
- [x] 3.4 `WhoAmI` carries the current tenant's icon

## 4. Surfaces

- [x] 4.1 Capability registry entry `tenant.icon`
- [x] 4.2 HTTP `GET /api/v1/tenants/{ref}/icon`, client method, regenerated OpenAPI
- [x] 4.3 CLI `--icon`, `--icon-name`, `--icon-file` on `tix tenant edit`, `tix tenant icon`, table columns
- [x] 4.4 Web picker on the tenant settings screen, icon route, switcher rows, sidebar entry and header
- [x] 4.5 TUI memberships behind a text stand-in
- [x] 4.6 i18n keys in every catalog

## 5. Docs and tests

- [x] 5.1 Docs for the API, CLI and web UI
- [x] 5.2 Store tests on both engines, service, HTTP, CLI and web tests
