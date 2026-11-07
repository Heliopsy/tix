# Tasks

Sizes: S under a day, M one to three days, L three to five days. Ships after #216. Total roughly 5 to 7
weeks of focused work, in four pull requests merged in order.

## 1. Account to actor link, no behaviour change (PR 1, M+L)

- [ ] 1.1 Migration 0035: backfill `actors.user_id`, partial unique `(tenant_id, user_id)`, index on
  `user_id`, `sessions.second_factor` and `sessions.method` (S)
- [ ] 1.2 Both stores write `actors.user_id` on every user actor created (`CreateUser`, tenant bootstrap,
  invites) and read `actors.disabled_at` (S)
- [ ] 1.3 `UserOfActor` / `ActorOfUser` store methods; unscoped `ListMemberships(userID)` on the allow-list
  with its isolation guard, both engines (M)
- [ ] 1.4 Replace every `GetUser(ctx, actor.ID)` and `GetActor(ctx, user.ID)`: `session.go`, `user.go`,
  `globaladmin.go`, `totp.go`, `sshlogin.go`, `token.go`, invites (M)
- [ ] 1.5 Guard: a person whose actor ID differs from the user ID signs in, passes a second factor, signs in
  over SSH, mints a token, is a global admin and is administered; watched failing on the old code (M)

## 2. Membership and account rules (PR 2, L)

- [ ] 2.1 `AddTenantMember` for global admins (service, registry, HTTP, CLI `tix tenant user add`, web
  Global admin area); tenant bootstrap accepts an existing account as first admin (M)
- [ ] 2.2 Invites for an existing account: actor at issue, membership at acceptance, acceptance requires
  password and enrolled factor, pending row on the Users screen, not assignable until accepted (M)
- [ ] 2.3 Sole-tenant rule on password, email, require-password-change, factor reset, disable and delete,
  with `account_in_other_tenants`; per-tenant disable and removal for everyone else (M)
- [ ] 2.4 Removal revokes that actor's sessions, tokens and keys in the same transaction (S)
- [ ] 2.5 Verifiers read account and actor disabled state per request; global disable sweeps tenants (M)
- [ ] 2.6 Tenant deletion deletes accounts left with no actor; `tix doctor` lists orphans (S)
- [ ] 2.7 Snapshot import maps an existing email only for a global admin (S)

## 3. Switching and the shared host (PR 3, L)

- [ ] 3.1 `whoami` memberships, `account_id`, `account:read` scope (registry, OpenAPI, client) (S)
- [ ] 3.2 `resolveTenant`: credential and `X-Tix-Tenant` on the shared host, refusal on a bound host (M)
- [ ] 3.3 Same-host switch: `POST /switch`, factor challenge, new session, old one ended, audit (M)
- [ ] 3.4 Migration 0036 `switch_tickets`; cross-host ticket mint and `POST /switch/redeem` page with the
  confirm button; log redaction (M)
- [ ] 3.5 Web header switcher dialog, `?dialog=tenants` fallback, Bulgarian strings, screenshots (M)

## 4. CLI, TUI, SSH, docs (PR 4, M)

- [ ] 4.1 `tix login --tenant`, `tix tenant ls|use` from memberships, `tix whoami` lists them (M)
- [ ] 4.2 TUI tenant view lists memberships and switches where a credential exists (S)
- [ ] 4.3 `tix ssh key add --all-tenants` (S)
- [ ] 4.4 `docs/tenancy.md`, `docs/web-ui.md`, `docs/api.md`, skill, release note on rollback (S)
- [ ] 4.5 Archive `global-admins-and-tenant-bootstrap`'s "Existing users" limitation (S)
