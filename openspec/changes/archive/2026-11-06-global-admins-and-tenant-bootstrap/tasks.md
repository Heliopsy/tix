# Tasks

## 1. Contract and stores

- [x] 1.1 Migration 0030 `users.global_admin`, both stores read it, `SetUserGlobalAdmin` writes it
- [x] 1.2 `core.ScopeGlobalAdmin`, not implied by `*`; `core.User.GlobalAdmin`; `core.Actor.GlobalAdmin` on whoami
- [x] 1.3 `CreateTenantInput.Admins`, `CreatedTenant`, `InitialCredential`, `TenantUser`, `GlobalAdminInput`

## 2. Authorization

- [x] 2.1 `authz.Policy.CanGlobal`, `Can` refuses `global.admin`
- [x] 2.2 Direct-store marker set only by the local connection without a token
- [x] 2.3 Token minting with `global:admin` checks the minter and the token's actor

## 3. Service

- [x] 3.1 `CreateTenant` global admin only, with first admins, generated passwords, unwind on failure
- [x] 3.2 `GrantGlobalAdmin`, `RevokeGlobalAdmin` (last one refused), `ListGlobalAdmins`, audited
- [x] 3.3 `ListTenantUsers`, `CreateTenantUser`, `UpdateTenantUser`, `DeleteTenantUser`
- [x] 3.4 `ListTenants`, `GetTenant`, `UpdateTenant`, `DeleteTenant` reach any tenant for a global admin

## 4. Surfaces

- [x] 4.1 Registry entries, HTTP routes, client, `just openapi`
- [x] 4.2 CLI `tix global-admin`, `tix tenant create --admin --name`, `tix tenant user`
- [x] 4.3 Web Global admin area, one-time passwords, per-tenant users, Bulgarian strings
- [x] 4.4 MCP overlay marks the authority-granting tools destructive; global tools need whoami's flag

## 5. Docs and tests

- [x] 5.1 docs/tenancy.md, docs/api.md, docs/mcp.md, docs/agents.md, skill
- [x] 5.2 Service, HTTP, CLI, web, integration matrix and authz tests; each new guard watched failing
