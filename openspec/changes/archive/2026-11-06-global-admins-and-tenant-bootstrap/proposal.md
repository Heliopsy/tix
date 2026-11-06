# Global admins, and tenants created with their first admins

## Why

`POST /api/v1/tenants` created an empty tenant: no members and no admin. Every other call acts on the
caller's own tenant, so nobody could sign in to a tenant created over the API or add a user to it. The
only way in was the command line against the database on the server. At the same time any tenant's admin
could create tenants, which is installation administration handed to whoever runs one tenant.

## What Changes

- **Global admin**, a flag on the user (`users.global_admin`, migration 0030). A global admin creates
  tenants, lists, renames and deletes any tenant, and manages the users of any tenant. The first one is
  granted from a shell holding the database: `tix --db PATH global-admin grant EMAIL`. After that a global
  admin grants and revokes others over the API, the command line and the browser. The last one cannot be
  revoked. Grants and revokes are audited.
- **BREAKING**: creating a tenant needs a global admin. A tenant admin, and a token holding `*`, can no
  longer create one. Upgrade step: grant the first global admin from the shell before relying on tenant
  creation over the API.
- New scope `global:admin`. `*` does not imply it. A token carries it only when named, only when minted by
  a global admin for an actor who is one, and it works only while that user is still a global admin.
- `CreateTenantInput.admins`: first admins, each created as a user holding the admin role in the new tenant
  only. A password left out is generated and returned once in `initial_credentials`; it is never stored in
  plain text and never written to the audit log, the event stream or a log. An address that already belongs
  to a user is refused before anything is written.
- Routes: `GET|POST /api/v1/global-admins`, `DELETE /api/v1/global-admins/{email}`,
  `GET|POST /api/v1/tenants/{ref}/users`, `PATCH|DELETE /api/v1/tenants/{ref}/users/{id}`. `GET`, `PATCH` and
  `DELETE /api/v1/tenants/{ref}` and `GET /api/v1/tenants` reach any tenant for a global admin.
- CLI: `tix global-admin grant|revoke|ls`, `tix tenant create KEY [NAME] --name N --admin EMAIL...`,
  `tix tenant user ls|create|edit|rm KEY ...`.
- Web: a Global admin area under Admin, offered only to a global admin: every tenant with create, rename and
  delete, the generated passwords shown once with copy buttons, the global admins with grant and revoke,
  and each tenant's users.
- MCP: the new tools appear only when the token names `global:admin` and whoami reports a global admin;
  the ones that hand out authority are destructive and opt-in.

## Capabilities

### Modified Capabilities

- `multi-tenancy`: global admins, tenant creation with first admins.
- `http-api`: the global admin and tenant user routes.
- `cli`: the global-admin and tenant user commands, `--admin` on tenant create.
- `web-ui`: the global admin area.

## Impact

`core.Service.CreateTenant` returns `*core.CreatedTenant`, which embeds `core.Tenant`, so the JSON keeps
the tenant's fields at the top level. Migration 0030 on both engines.
