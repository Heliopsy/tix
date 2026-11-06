# Design

## Who may create a tenant

Before this change `CreateTenant` was gated on `tenant:admin`, which every tenant's admin holds. Any
tenant admin could therefore create tenants, which is installation administration. This change narrows it
deliberately: only a global admin may create a tenant. Nothing else a tenant admin could do is widened or
narrowed; their reach stays their own tenant.

A global admin is a flag on the user row (`users.global_admin`), not a membership role. Roles are per
tenant (viewer, member, admin; there is no owner), and this one is about the installation. It defaults
off, so an upgrade grants nobody anything.

## How the authority is decided

`authz.Policy.CanGlobal` decides, never `Can`; `Can` refuses `global.admin` outright, so no scope alone
reaches it. The service tells the policy two facts it looked up:

- **Direct store.** A process that opened the database itself with no token stands as a global admin.
  Whoever can open the file already holds every row in it. Only `internal/connect`'s local connection sets
  this, through a context marker nothing on the network can set. This is how the first global admin is
  granted, and why `tix --db PATH tenant create` keeps working as before.
- **The user is a global admin now.** Read from the store on every call, so a revoke reaches live
  sessions and tokens at once, and a disabled user is not one.

A token additionally needs `global:admin` named among its scopes. `*` does not carry it, so no token minted
before this change gained authority over other tenants. Minting a token with it needs the minter to be a
global admin and the token's actor to be one; the tenant-admin rule that a creator may only grant scopes it
holds still applies to every other scope. A project-pinned token is never a global admin.

## Reaching another tenant

The tenant-confined user methods are reused rather than duplicated. `inTenant` checks global admin first,
before the reference is looked at, so a tenant admin learns nothing about which tenants exist; then it
resolves the tenant and runs the ordinary method with the caller standing in that tenant as its admin.
Every query stays scoped to that tenant by the builder, and the audit entries there name the global admin
who acted.

`GetTenant`, `UpdateTenant` and `DeleteTenant` accept any tenant for a global admin and keep answering
`not_found` for anybody else. `ListTenants` lists every live tenant for a global admin. The web area does
not offer deleting the reader's own tenant.

## Tenant creation with first admins

The tenant row and its audit entry are written in the caller's tenant, as before. The builtin workflow and
the admins (user, actor, admin membership, audit entry and event each) go in one transaction scoped to the
new tenant, because a transaction is bound to one tenant and on PostgreSQL row-level security holds it
there. If that second transaction fails, the tenant row is removed again in the caller's tenant with a
`tenant.delete` entry giving `reason: setup_failed`, so a failure never leaves a tenant nobody can sign in
to and a retry with the same key works. If removing it fails too, the error says so and names the key.

Validation runs before anything is written: a valid email for each admin, no address twice, no two admins
deriving the same handle, a supplied password of at least 12 characters, and at most 10 admins, which
bounds the password hashing one request can buy. Passwords are hashed before any transaction opens.

A generated password is 24 characters from `crypto/rand` over an alphabet without 0 O o 1 l I, grouped in
sixes with hyphens. It is returned once in `initial_credentials` and nowhere else: the audit entry and the
event record the user and `credential: generated` or `supplied`, never the value.

TODO: once the separate change adding "require a password change at first sign-in" lands, set that flag on
every admin whose password was generated here.

## Existing users

A user row is global, but a user and its actor share one identifier and an actor lives in exactly one
tenant, so in this version a user belongs to one tenant. Sign-in, user administration and sessions all rely
on that. Making an existing user an admin of a second tenant would need a second actor pointing at the same
user, and every one of those paths taught to follow it. It would also let the admins of one tenant reset
the password of a person who administers another, which is a way into the second tenant. So an address
that already belongs to a user is refused with a conflict naming it, before anything is written, and the
caller names another address. Multi-tenant identities are a follow-up of their own.

For the same reason there is no `include_caller`: the caller cannot be a member of the new tenant.

## Admins of the new tenant cannot reach the caller's

They are members of the new tenant only. Sign-in to the caller's tenant with their address finds no actor
there and fails the same way a wrong password does. They are not global admins unless one grants it.

## Web

A Global admin entry under Admin, drawn only when whoami reports a global admin, whatever the advanced
screens preference says. The generated passwords travel to the next load of the screen in an HttpOnly,
SameSite=Strict cookie scoped to `/admin/global` that lives 60 seconds and is cleared as it is read, the
pattern the token screen already uses, so they never appear in a URL or the audit trail.

## MCP

The tools follow from the spec. A tool needing `global:admin` is offered only when the token names it and
whoami reports a global admin. Tenant creation, granting global admin, and creating or changing a user in
another tenant are not deletes but hand out authority over other tenants, so the overlay marks them
destructive: offered only with `--allow-destructive` or an overlay that enables them. Revoking and deleting
are DELETEs and destructive already.

## Last global admin

Revoking the last enabled global admin is refused with `precondition_failed` and `reason:
last_global_admin`. On PostgreSQL two concurrent revokes under read committed could each see the other
still present; the shell holding the database can always grant again, which is the recovery.
