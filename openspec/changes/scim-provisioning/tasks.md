# Tasks

## 1. Spec

- [ ] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Storage

- [ ] 2.1 Migration adding the `scim_*` tables, scoped and row-level secured
- [ ] 2.2 SQLite and PostgreSQL read and write them

## 3. Contract and service

- [ ] 3.1 `core.ScopeSCIMProvision`, `core.ViaSCIM`, SCIM types and `core.Provisioner`
- [ ] 3.2 Provisioning: users, groups, linking, deactivation, deletion, role mapping, audit
- [ ] 3.3 Administration: show, enable, disable, token, map and unmap a group
- [ ] 3.4 The token verifier confines a provisioning token to `/scim/v2`

## 4. Surfaces

- [ ] 4.1 `internal/scim`: discovery, Users, Groups, filters, PATCH, errors
- [ ] 4.2 HTTP admin routes, regenerated OpenAPI, client
- [ ] 4.3 CLI `tix tenant scim`
- [ ] 4.4 Web SCIM screen and i18n in every catalog
- [ ] 4.5 Terminal interface exemption recorded in the registry

## 5. Docs and tests

- [ ] 5.1 `docs/scim.md` and the API, configuration, README, ROADMAP and skill updates
- [ ] 5.2 Store, service, protocol, CLI, HTTP and web tests
