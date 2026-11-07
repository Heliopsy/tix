# Design

## Where things stand

| Table | Scope | Keyed on | Note |
| --- | --- | --- | --- |
| `users` | global | `id`, `email` unique | password, sso, `global_admin`, `disabled_at`; #216 adds `require_password_change`, time prefs |
| `user_totp`, `user_recovery_codes` | global | `user_id` | already per person, not per tenant |
| `actors` | tenant | `id`, `(tenant_id, handle)` | `user_id` and `disabled_at` exist, neither is written or read |
| `tenant_members` | tenant | `(tenant_id, actor_id)` | role |
| `sessions`, `api_tokens`, `ssh_keys` | tenant | `actor_id` | `ssh_keys` unique on `(tenant_id, fingerprint)` |
| `user_invites` (#216) | tenant | `user_id` references `actors(id)` | the column is an actor ID despite its name |
| stars, saved filters, inbox, group prefs, calendar feeds, snoozes, templates | tenant | `actor_id` | already per tenant, fine as is |

User and actor are written with the same ID (`CreateUser`, tenant bootstrap, invites), and these read the
user through the actor's ID: `findCandidate` and `completeLogin` (`session.go`), `ownUser` and `ListUsers`
(`user.go`), `isGlobalAdmin` (`globaladmin.go`), `totp.go`, `enrolledPerson` (`sshlogin.go`) and the token
target check (`token.go`). That coincidence is the whole "one tenant per user" rule.

## Options

**A. One global account, one actor plus membership per tenant (recommended).** `users` is the account,
`actors.user_id` links each tenant's actor to it. This is what the baseline `multi-tenancy` spec already
describes, what `user_totp` (migration 0011) and `ssh_keys` (0005) were written to expect, and the
smallest step from the current schema: no new table, no ID rewrite.

**B. A separate identity table.** Add `identities` (email, password, factor) and leave `users` per tenant.
It means moving the password, second factor and global admin flag out of `users`, rewriting 0011's keys,
and two notions of "user" in the code. It buys nothing A does not, since `users` is already global.
Rejected.

**C. Separate accounts per tenant with linked sign-in.** Each tenant keeps its own `users` row (email
unique per tenant, not globally), and a link lets one sign-in reach the linked accounts. Rejected:

- Email uniqueness moves to `(tenant, email)`, so sign-in on a shared host has to ask which account, and
  every lookup by email changes.
- Two passwords and two second factors drift; the person has to keep both up and the link has to say
  which one counts. If a link lets one credential open the others, the weakest tenant's admin can reset
  that tenant's copy and walk through the link, which is exactly the takeover this change has to close.
  If it does not, the link is a bookmark.
- Global admin, `user_totp` and the time preferences are per person today and would have to be copied.
- More code than A for less: two account models, a link table, link consent, unlink.

## Data model (A)

- `users` unchanged in meaning: the account.
- `actors.user_id` set for every `kind = 'user'` actor. A partial unique index on `(tenant_id, user_id)`
  where `user_id IS NOT NULL` keeps one actor per person per tenant. An index on `user_id` serves the
  membership listing.
- `actors.disabled_at` becomes the per-tenant disable. `users.disabled_at` stays the account-wide one.
- `tenant_members` unchanged.
- `sessions` gains `second_factor` (whether the sign-in that minted it passed one) and `method`
  (`password`, `ssh`, `switch`), so a switch can be checked against the target's policy.
- New `account_log`: `seq`, `user_id`, `action`, `tenant_key` (the tenant the change was made from, or
  that a membership change concerns), `by_user_id`, `by_global_admin`, `via_kind`, `detail` (JSON, never a
  secret), `occurred_at`. Unscoped, on the allow-list, keyed on the account. See "Account log".
- Later phase: `switch_tickets`: `id`, `user_id`, `target_tenant_id`, `audience`, `token_hash`,
  `second_factor`, `created_at`, `expires_at`, `used_at`. Unscoped, on the allow-list, like `user_totp`,
  because it is written in one tenant and redeemed in another.
- `user_invites` from #216 is reused. For an address that already has an account, issuing creates the actor
  (so the row's `user_id` foreign key holds) but no membership; acceptance adds the membership. #216
  already distinguishes `existing_account`.

### IDs

Existing rows keep their IDs, so for every person today `actor.id == user.id`. New actors get new IDs.
In tenant-scoped APIs a user's `id` stays the actor ID in that tenant (unchanged meaning: core.User.ID is
documented as "the user's actor identifier"), and `account_id` is added. Everything keyed on actor ID
(tasks, comments, audit, stars, tokens, keys) is already per tenant and needs nothing. Everything keyed on
user ID (`user_totp`, recovery codes, global admin, password) reaches the user through `actors.user_id`.

A store helper `UserOfActor(actorID)` and `ActorOfUser(userID)` (the latter scoped) replace every
`GetUser(ctx, actor.ID)`. A test seeds a person whose actor ID differs from the user ID and runs sign-in,
second factor, SSH, token minting, global admin and user admin through it; that is the guard that the
coincidence is gone, and it must be watched failing against today's code.

### Migrations

Numbered after the highest on `main` once #216 has merged (0031 to 0034 are #216's). If the switch to
timestamp-named migrations lands first, they are made with `just migration NAME` instead. Below they are
called by what they do.

- link: `UPDATE actors SET user_id = id WHERE kind = 'user' AND user_id IS NULL AND id IN (SELECT id FROM
  users)`; the partial unique index; the `user_id` index; `sessions.second_factor` (default 0) and
  `sessions.method` (default `password`).
- account log: `account_log` and its index on `(user_id, seq)`.
- later phase: `switch_tickets`.
- Users with no actor (orphans left by tenant deletion before this change) are left alone and listed by
  `tix doctor`; a global admin can delete them. Not deleting them silently keeps the migration pure.
- Postgres RLS: the cross-tenant membership listing, `account_log` and `switch_tickets` go through `Unscoped`, named on
  the allow-list in `internal/store/sql`, and the isolation guard is extended for them.
- No down migration, as with every other one here. Rolling back the binary after the link migration is safe as long as no
  second actor was created: older code ignores `actors.user_id`. Once a person has two actors, an older
  binary finds them only in the tenant whose actor ID equals the user ID. The release notes say so.

## Sign-in and tenant switching

### One tenant per session

A session stays one row with one `tenant_id` and one `actor_id`. Every check that exists today keeps
working: the credential's tenant must equal the resolved tenant, RLS pins the transaction, logout ends one
row, revalidation re-reads one actor. A multi-tenant session (one cookie, tenant per request) would need
each of those reworked and would let a stolen cookie reach every tenant. Rejected.

Sign-in is as today: the host picks the tenant, `findCandidate` finds the account by email and then the
person's actor in that tenant through `actors.user_id`. No actor or no membership there is the same
"invalid credentials" as a wrong password, so sign-in never says which tenants an address belongs to.

### Switching

`POST /switch` with the target tenant key, from an authenticated session:

1. Look up the caller's account and its actor and membership in the target. No membership, a disabled
   actor or account: refused as not found (the same answer for a tenant that does not exist).
2. Target requires a second factor on the web and the current session has `second_factor = 0`: the
   person is challenged for a code before anything is minted, the same challenge sign-in uses.
3. Target served on the same host (the shared host): mint a session for the target actor, set the cookie,
   end the old session. Recorded in the target as `user.login` with method `switch`.
4. Later phase, not in the first PRs: target has its own verified domain: mint a switch ticket (32 random bytes, stored hashed, 60 seconds,
   single use, bound to the account, the target tenant and the target host) and answer with a page whose
   one form POSTs it to `https://TARGET/switch/redeem`. Redeeming re-checks step 1, mints the session there
   and marks the ticket used in the same transaction. The source session is left alone, because it lives
   on another host's cookie.

The redeem page shows "Continue to TENANT as EMAIL" with a button and does not auto-submit. That works
without scripting and makes a forced sign-in (an attacker posting their own ticket into a victim's
browser) visible instead of silent. The ticket never appears in a URL; the redeem route is redacted from
the request log anyway, as invite routes are.

### The shared host

A host that no tenant domain claims resolves today to `DefaultTenantID`. On that host only, the tenant may
also come from the credential: a session cookie or token of tenant T is served as tenant T, and the
`X-Tix-Tenant` header names a tenant for sign-in and for a client that holds several contexts against one
URL. The header is read by the transport in `resolveTenant`, once per request, never by a service, which
is what the "Tenant resolved once per request" requirement asks. On a host bound to a tenant domain the
header must match or be absent, and a credential of another tenant is refused as today. That keeps the
cookie isolation that per-tenant domains give, and lets an installation with one hostname host several
tenants, which the `test` case probably needs.

## Tokens and SSH keys

Tokens stay minted for one tenant actor, which is the baseline "API tokens are tenant-scoped" rule. A
token per account with a tenant picked per request would be one secret reaching every tenant the person is
in, including tenants added after it was minted. The only cross-tenant credential stays a global admin's
token with `global:admin` named. A person with three tenants holds three tokens, one per CLI context.

SSH keys stay enrolled per tenant actor. A person enrols the same public key in each tenant they want to
reach (the unique is already per tenant), and the username picks the tenant as `ssh-access` already
specifies. Self-service enrolment gains `--all-tenants`, which writes one enrolment per membership, each in
its own tenant transaction with its own audit entry. That is a convenience, not a new credential.

Removing a person from a tenant revokes their sessions, tokens and keys in that tenant only, in the same
transaction as the membership removal.

## Who may add a person to a tenant

| Who | How | Consent |
| --- | --- | --- |
| global admin | `AddTenantMember(tenant, email, role, handle)` on an existing account | none needed; a global admin can already set the account's password |
| tenant admin | invite (#216); for an existing account, the actor is created at issue, the membership at acceptance | the person accepts with their own password and, if enrolled, their second factor |
| tenant admin | `CreateUser` with an address that has an account | refused with the conflict it gives today; never attaches |
| the person | none; nobody joins a tenant on their own | |

The invite link holder is not trusted. For an existing account the link alone does nothing: acceptance
asks for the account's password and second factor, so the admin who copied the link cannot accept it for
the person. A pending existing-account invite shows on the target's Users screen with its email and no
role, and the actor is not offered as an assignee until accepted.

For the concrete case: a global admin runs `tix tenant user add test villy@... --role admin` and the same
for icek@. Their next sign-in at HO shows `test` in the switcher.

## Who may change an account

The risk: anything a tenant admin can do to the account (not to the tenant's actor) reaches every tenant
the person is in.

| Change | Self | Global admin | Tenant admin, person only in this tenant | Tenant admin, person also elsewhere |
| --- | --- | --- | --- | --- |
| password, require password change | yes | yes | yes (today's behaviour) | no |
| email | yes | yes | yes | no |
| clear second factor (`totp:admin`) | n/a | yes | yes | no |
| disable account | no | yes | yes, disables the account | no; disables the actor in this tenant |
| delete | no | yes | yes, deletes the account | no; removes from this tenant |
| role, handle, actor display name | | yes | yes | yes, in this tenant |
| tokens, SSH keys of the tenant's actor | own | yes | yes | yes, in this tenant |

"Only in this tenant" is decided in the service, inside the write transaction, from the unscoped
membership listing. The refusal is `forbidden` with reason `account_in_other_tenants` and says to ask the
person or a global admin; it does not name the other tenants. It is the one place a tenant admin learns
the person has another membership, and it is unavoidable: any rule that treats such people differently
reveals that. The alternative strict rule (no tenant admin ever changes a password) breaks today's
single-tenant deployments, where the tenant admin is often the only recovery path. Asked below.

The password-change-at-first-sign-in flag from #216 is an account change and follows the same column.

## Global disable

A disabled account must stop everywhere at once, but a transaction is bound to one tenant. So: set
`users.disabled_at` first (unscoped, one write), and make the session and token verifiers read the
account's and the actor's disabled state on each request (one indexed join they already almost do for the
role). Then revoke sessions, tokens and keys tenant by tenant, each with its audit entry. A failure part
way leaves credentials that are already refused by the verifier, and a retry finishes the sweep.

## whoami and the switcher's data

`whoami` gains `account_id`, `email` and `memberships`: `tenant_key`, `tenant_name`, `role`, `actor_id`,
`handle`, `host` (the tenant's primary verified domain, empty for the shared host). Listed for a session,
and for a token only when it carries the new `account:read` scope, which `*` does not imply. An agent's
token in tenant A should not tell its holder which other tenants its owner is in.

## Audit and via

- Every entry stays in one tenant and names the actor in that tenant. Nothing in tenant B names the
  person's actor or tenant in A.
- A switch is `user.login` in the target with `method: switch` and via `web`; the source records nothing,
  so the source admins do not learn where the person went.
- Membership added: `member.add` in the target, by the global admin (standing in it through `inTenant`, as
  today) or `invite.issue` by the tenant admin followed by `invite.accept` by the person.
- Account changes (password, email, second factor) are recorded in the tenant the change was made from, as
  today, and in the account log. No fan-out to the person's other tenants, which would need a write in each
  tenant per change and would tell each of them the person is elsewhere.
- A global admin acting in another tenant keeps the via detail #216 records, plus `global_admin: true`.

## Account log

Per-tenant audit cannot answer "what happened to my account", because the changes are spread over the
tenants they were made from and the person may not read audit in all of them. The account log is one
list per account, readable by the person and by global admins, and by nobody else: a tenant admin never
reads it, since it names the person's other tenants.

- Written in the same transaction as the change it records. The `users` row is unscoped and already
  written inside a tenant transaction, so the log row goes in that same transaction, which keeps the
  "rows, audit, event in one transaction" rule.
- Recorded: password set or changed, required password change set or cleared, email changed, second factor
  enrolled, cleared or recovery codes regenerated, account disabled or enabled, global admin granted or
  revoked, membership added (direct or invite accepted), role changed, membership removed, account deleted
  (kept for global admins, the person can no longer read it). Sign-ins and switches are not recorded: they
  are per-tenant events already and would drown the rest.
- Each row names who acted (the person, a tenant admin by email, or a global admin), the tenant key it came
  from or concerns, and the via kind from #216. Never a password, hash, code or token.
- Keyset paginated on `seq`. Retention follows the installation's audit retention.
- Surfaces: `GetAccountLog` (self) and `GetAccountLogFor(email)` (global admin), with registry entries;
  `tix account log`, `tix global-admin account-log EMAIL`; `GET /account/log`,
  `GET /accounts/{email}/log`; a web Account screen reached from Settings (and from the header's tenant dialog once
  that exists), and a link from each user in the Global admin area.

## CLI

- A context stays one server, one token, one tenant. `tix login --server URL --tenant KEY [--context
  NAME]` signs in on the shared host with `X-Tix-Tenant` and stores a token for that tenant's actor; on a
  tenant domain the URL decides and `--tenant` must agree.
- `tix tenant ls` without global admin lists the caller's memberships from whoami (today it is the caller's
  own tenant). `tix tenant use KEY` checks the key against those memberships instead of probing, and says
  `tix login --tenant KEY` when the current context has no credential there.
- Local (`--db`) use is unchanged: the direct-store marker already stands as a global admin.
- `tix whoami` prints the memberships.

## TUI

The tenant view lists memberships from whoami and switches to one with a credential for it (local, or a
context for that tenant). The "no list is offered" rule goes, since the reason it gave no longer holds.

## Web

The header shows the current tenant's name and colour as a button. It opens a dialog listing the person's
tenants, each with its role and a Switch button (a form POST to `/switch`), the current one marked and
without a button. Without scripting the button is a link to `?dialog=tenants`, as other dialogs are. With
one membership the name is plain text and there is no dialog. No `<details>`. A global admin also gets a
link from the dialog to the Global admin area; tenants they are not a member of are not in the switcher.

## What breaks and compatibility

- Behaviour for existing deployments: none until someone is added to a second tenant. Every person is in
  one tenant, so every admin power a tenant admin has today still applies to them.
- API: additive. `id` on users keeps its meaning; `account_id`, `memberships`, the switch routes and the
  header are new. `account:read` is a new scope.
- `CreateUser` and tenant creation with an existing address: still refused for tenant admins. Tenant
  creation by a global admin with an existing address as first admin becomes allowed and adds a
  membership instead of refusing, which retires the "Existing users" paragraph of
  `global-admins-and-tenant-bootstrap`.
- Snapshot import (`transfer`) that carries a user whose email has an account elsewhere: the import maps
  it to that account only when a global admin runs it; otherwise it is refused, never attached.
- Tenant deletion leaves accounts whose only actor was in it; the deletion now deletes such accounts in
  the same step (not global admins), and keeps accounts that are still members elsewhere.
- Rolling the binary back after a second membership exists: see the migration section.

## Security review by path

| Path | Threat | Answer |
| --- | --- | --- |
| tenant admin resets a multi-tenant person's password, email or factor | takeover of the person's other tenants | refused unless the person is in this tenant only |
| tenant admin invites an existing account | silent attach, or admin accepts their own copied link | membership only on acceptance with the person's password and factor |
| tenant admin `CreateUser` with an existing email | attach without consent | refused as conflict, as today |
| enumeration of who is in which tenant | admins of A learn about B | sign-in failure is uniform; memberships only listed to the person; the one leak is the `account_in_other_tenants` refusal, a yes/no with no names |
| sign-in on host A by a person only in B | cross-tenant session | no actor in A, same failure as a wrong password |
| switch to a tenant requiring a factor from a session without one | bypassing the target's policy | challenged before minting; `second_factor` recorded per session |
| switch ticket theft or replay | session in the target | 60 s, single use, hashed, bound to account, tenant and host, POST body only, never logged |
| forced switch (attacker's ticket in a victim's browser) | victim works in attacker's account | redeem page names the account and needs a click |
| removed membership with a live session or token | access after removal | credentials of that actor revoked in the same transaction; verifier refuses an actor with no membership |
| global disable racing per-tenant sweeps | credential still works | verifier reads both disabled flags per request |
| `X-Tix-Tenant` on a bound host | host isolation bypass | must match the host's tenant or be absent |
| token with `*` | reaching other tenants or listing them | tokens stay per tenant; memberships need `account:read`; `global:admin` unchanged |
| SSH key enrolled by a tenant admin on a person's actor | cross-tenant SSH | enrolment is per tenant; username picks the tenant; a key in B never opens A |
| global admin adds themselves anywhere | silent self-grant | allowed (they can already act in any tenant), audited in the target and in the account log |
| account log read by a tenant admin | learning the person's other tenants | readable only by the person and global admins |

## Implementation notes (PR 2)

- The cross-tenant check behind `account_in_other_tenants` reads the membership listing before the write
  that acts on it, not inside it: the listing crosses tenants and the write is bound to one. A membership
  granted between the two is granted by a global admin or by the person, and neither is the threat.
- On PostgreSQL the listing reads through SELECT-only policies on `actors` and `tenant_members`, raised by
  a transaction-local flag (`tix.account_lookup`), on the same terms as the SSH and domain lookups. Only
  `membershipsOf` in the service calls it, which a test enforces.
- A global admin reaching into a tenant through `inTenant` stands there without being a member, so the
  context carries a marker that the standing was checked; the reach decision reads it.
- An invite to an account of another tenant that holds a confirmed second factor asks for the code on the
  invite page itself, checked before anything is written, rather than through a second-step ticket after
  the membership exists. An account of another tenant with no password cannot be invited at all, because
  accepting would set its password.
- A tenant admin renaming a person who is also elsewhere renames their actor in this tenant only.
- Removing a person revokes their SSH keys here too, not only sessions and tokens.
- Global disable needs no sweep across tenants: the session and token lookups read the account's and the
  actor's disabled state on every request.

## Implementation notes (PR 3)

- `account:read` arrives with the account log rather than with whoami: a token reading its owner's log needs
  it. It is not implied by `*`, a person mints it only for their own actor, a token without it cannot mint
  one with it, and a shell holding the database may mint it for anybody.
- The log lists oldest first by default, like every other listing; the browser screen and the CLI ask for
  newest first.
- Writing the log needs the account behind the acting actor. A global admin acting through `inTenant` is not
  an actor of the target tenant, so the standing marker now carries the global admin's account; tenant
  creation carries it into the setup transaction, which runs as the system actor.

## Decisions

The owner answered the open questions of the first draft:

1. Tenant admins keep password and second factor reset for people who are members of their tenant only.
2. Tenant `test` shares HO's hostname for now. Shared-host resolution and the same-host switch come first;
   the cross-host switch ticket is a later phase, outside the first four pull requests.
3. A global admin's direct add needs no acceptance; it is audited.
4. The per-account change log is part of this change, readable by the person and global admins.
5. Tokens list memberships only with `account:read`, which `*` does not imply.
6. Removing the last membership deletes the account.
