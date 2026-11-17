# SCIM 2.0 provisioning

## Why

A team that runs its people through an identity provider (Entra ID, Okta, Authentik, JumpCloud,
Keycloak with a SCIM plugin) has to create, rename, deactivate and remove tix users by hand today,
tenant by tenant. Somebody who leaves the company keeps working sessions in tix until an admin
remembers them. SCIM 2.0 (RFC 7643 and RFC 7644) is what those providers speak to push that
lifecycle into an application, so tix should answer it.

Sign-in is a separate concern. A parallel change adds OIDC sign-in and owns sessions, the
`sso_providers` and `user_identities` tables and the `users.sso_*` columns. This change is
provisioning only and keeps its own data in `scim_*` tables.

## What Changes

- `tix serve` answers SCIM 2.0 under `/scim/v2`, scoped to the tenant the request resolves to the
  same way the API resolves one: by host, or on a shared host by the bearer token.
  - Discovery: `ServiceProviderConfig`, `ResourceTypes`, `Schemas`.
  - `Users`: list with `filter` (`userName eq`, `externalId eq`, `emails[type eq "work"].value eq`,
    `emails.value eq`, `id eq`, joined with `and`), `startIndex` and `count`; get, create, replace,
    patch (`add`, `replace`, `remove`, including `active`) and delete.
  - `Groups`: the same operations, with member `add` and `remove` over PATCH.
  - Errors use the SCIM error schema and `application/scim+json`.
- A new scope, `scim:provision`, is the only credential the endpoints take. It is carried only when
  named (`*` and the admin role do not imply it), a token carrying it carries nothing else, and such
  a token is refused everywhere outside `/scim/v2`.
- A tenant turns SCIM on and off. While it is off the endpoints answer 404.
- Admin operations on every surface: `tix tenant scim show|enable|disable|token|map-group|unmap-group`,
  the matching `/api/v1/scim` routes, a SCIM screen under Manage in the browser. Enabling creates
  the tenant's `scim` agent and mints its token once.
- `active=false` disables the person in this tenant and ends their sessions; their tokens and SSH
  keys stop working while they are inactive and work again when they are reactivated. Tasks and
  history are kept. DELETE removes the membership and revokes those credentials, never the history.
- A SCIM Group is a tenant group stored in `scim_groups`. A group's display name can be mapped to a
  tenant role, and a provisioned person's role follows the highest role mapped to their groups.
- Every SCIM change is audited with via kind `scim` and the token name.

## Impact

- One migration adds `scim_config`, `scim_users`, `scim_groups`, `scim_group_members` and
  `scim_group_roles`, all tenant scoped, row-level secured on PostgreSQL.
- `core.ViaSCIM`, `core.ScopeSCIMProvision`, the SCIM types and the `core.Provisioner` interface.
  Six new `Service` methods for administration, each with CLI, HTTP and web bindings; the terminal
  interface is exempt like the other tenant administration screens.
- New package `internal/scim` for the protocol. It is not in `api/openapi.yaml` and not an MCP
  tool: SCIM describes itself through its own discovery endpoints.
- Docs: new `docs/scim.md`; `docs/api.md`, `docs/configuration.md`, README, ROADMAP and the skill.
