# Tasks

## 1. Spec

- [x] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Storage

- [x] 2.1 Migration adding the `scim_*` tables, scoped and row-level secured
- [x] 2.2 SQLite and PostgreSQL read and write them

## 3. Contract and service

- [x] 3.1 `core.ScopeSCIMProvision`, `core.ViaSCIM`, SCIM types and `core.Provisioner`
- [x] 3.2 Provisioning: users, groups, linking, deactivation, deletion, role mapping, audit
- [x] 3.3 Administration: show, enable, disable, token, map and unmap a group
- [x] 3.4 The token verifier confines a provisioning token to `/scim/v2`

## 4. Surfaces

- [x] 4.1 `internal/scim`: discovery, Users, Groups, filters, PATCH, errors
- [x] 4.2 HTTP admin routes, regenerated OpenAPI, client
- [x] 4.3 CLI `tix tenant scim`
- [x] 4.4 Web SCIM screen and i18n in every catalog
- [x] 4.5 Terminal interface exemption recorded in the registry

## 5. Docs and tests

- [x] 5.1 `docs/scim.md` and the API, configuration, README, ROADMAP and skill updates
- [x] 5.2 Store, service, protocol, CLI, HTTP and web tests
