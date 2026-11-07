# One account in many tenants

## Why

One person should be able to work in several tenants with one email, one password and one second factor,
and move between them without signing out. The case that prompted this: villy@ and icek@ are in tenant HO
and should also administer tenant `test`.

Today they cannot. A `users` row is global and `user_totp` already keys on it, and the `multi-tenancy`
baseline already says "one identity, several tenants". The code does not do that. A user and its actor
share one identifier and an actor lives in one tenant, so a user belongs to exactly one tenant. Sign-in
(`findCandidate`), user administration (`ownUser`), global admin checks, second factor lookups, SSH sign-in
and token minting all find the user by the actor's ID. `actors.user_id` exists in the schema but nothing
writes it. Tenant creation and invites (#216) refuse an address that already has an account for that
reason, and because a tenant admin can set any of its users' passwords, which would let the admin of one
tenant take over a person who administers another.

## What Changes

- **One global account, one actor and membership per tenant.** `users` stays the account (email,
  password, second factor, global admin, global disable). A person gets one `actors` row per tenant, linked
  by `actors.user_id`, at most one per tenant. Existing rows keep their IDs; a migration backfills
  `actors.user_id`. No code may assume an actor ID equals a user ID afterwards.
- **Sessions stay bound to one tenant.** Sign-in picks the tenant from the host as today. A switcher mints
  a session for the person's actor in another tenant, checking that tenant's membership and second factor
  policy. On a host bound to a tenant domain, the switch hands off to the target tenant's host with a
  single-use ticket. A host no domain claims (the default-tenant fallback) serves whichever tenant the
  session or the `X-Tix-Tenant` header names, and only to a credential of that tenant.
- **Tokens and SSH keys stay per tenant actor.** No cross-tenant token is added. SSH keeps choosing the
  tenant by username.
- **Adding an existing person to a tenant:** a global admin adds them directly; a tenant admin only
  invites, and the membership exists only after the person accepts with their own password and second
  factor. Creating a user with an address that already has an account stays refused for tenant admins.
- **Account credentials are the person's.** Password, email, second factor reset, forced password change
  and global disable of a person who holds a membership in another tenant can be changed only by that
  person or a global admin. A tenant admin keeps those powers over a person who is in their tenant only,
  which is everybody today. Disable and remove become per tenant for a tenant admin.
- **`whoami` lists the caller's memberships** for interactive credentials, which drives the web switcher,
  `tix tenant ls`, the TUI tenant view and CLI context validation.
- **Web:** the current tenant is a header control opening a dialog of the person's tenants, each a
  Switch button. No `<details>`.
- **CLI:** one context per tenant, each with its own token. `tix login --tenant KEY` on a shared host;
  `tix tenant use` checks the key against the memberships.
- **Audit:** each tenant records what happened in it, by the person's actor in that tenant. A switch is
  recorded in the target tenant as a sign-in with method `switch`, never naming the source tenant.

## Capabilities

### New Capabilities

None. The behaviour extends existing capabilities.

### Modified Capabilities

- `multi-tenancy`: account to actor link, adding people to a tenant, removal per tenant, tenant switch,
  TUI and CLI selection from memberships.
- `auth`: who may change an account's credentials, administrative second factor reset, switch tickets,
  per-request disable checks.
- `http-api`: memberships on whoami, tenant chosen on the shared host.
- `web-ui`: header tenant switcher, invite acceptance for an existing account.
- `cli`: per-tenant contexts and login.
- `ssh-access`: keys stay per tenant actor, removal revokes them.
- `audit-log`: attribution across tenants.
- `data-model`: `actors.user_id` and the migration.

## Impact

- Migrations 0035 and 0036 (0031 to 0034 are taken by #216). Backfill only; no ID changes.
- `internal/service`: `session.go`, `user.go`, `tenantusers.go`, `globaladmin.go`, `totp.go`,
  `sshlogin.go`, `token.go`, `sshkey.go`, invites from #216, a new `switch.go` and `memberships.go`.
- `internal/store` (both engines): actor lookups by user, an unscoped membership listing on the allow-list,
  session columns.
- `internal/httpapi/middleware.go` tenant resolution, `internal/auth` switch tickets.
- `internal/web`: header, switcher dialog, `/switch` routes, Users screen.
- `cmd`: `login`, `tenant ls|use`, `ctx`, `whoami`, `tenant user add`.
- API: additive (`memberships` on whoami, `account_id` on users, new routes). `id` on a tenant's user stays
  the actor ID, which for every existing row is also the account ID.
- Depends on #216 (invites, required password change, audit `via`) being merged first.
