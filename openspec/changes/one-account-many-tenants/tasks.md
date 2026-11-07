# Tasks

Sizes: S under a day, M one to three days, L three to five days. Ships after #216. Roughly 6 to 8 weeks
for PRs 1 to 5, merged in order; the cross-host switch is a later phase.

## 1. Account to actor link, no behaviour change (PR 1, M+L)

- [x] 1.1 Link migration: backfill `actors.user_id`, partial unique `(tenant_id, user_id)`, index on
  `user_id` (S)
- [x] 1.2 Both stores write `actors.user_id` on every user actor created (`CreateUser`, tenant bootstrap,
  invites) and read it back on the actor (S)
- [x] 1.3 `GetActorByUser` store method (scoped, both engines) and the service's `accountOf`, which reads
  the account through `actors.user_id` (S)
- [x] 1.4 Replace every `GetUser(ctx, actor.ID)` and `GetActor(ctx, user.ID)`: `session.go`, `user.go`,
  `globaladmin.go`, `totp.go`, `sshlogin.go`, `token.go`, invites (M)
- [x] 1.5 Guard: a person whose actor ID differs from the user ID signs in, passes a second factor, signs in
  over SSH, mints a token, is a global admin and is administered; watched failing on the old code (M)

## 2. Membership and account rules (PR 2, L)

- [x] 2.0 Unscoped `ListMemberships(userID)` on the allow-list with its isolation guard, both engines; the
  first reader is the sole-tenant rule (M)
- [x] 2.1 `AddTenantMember` for global admins, no acceptance (service, registry, HTTP, CLI
  `tix tenant user add`, web Global admin area); tenant bootstrap accepts an existing account as first
  admin (M)
- [x] 2.2 Invites for an existing account: actor at issue, membership at acceptance, acceptance requires
  password and enrolled factor, pending row on the Users screen (M)
- [ ] 2.2a A pending invitee is not assignable until accepted. Assignees are not membership-checked for any
  actor today, so this is a new rule for every actor rather than part of the invite (S)
- [x] 2.3 Sole-tenant rule on password, require-password-change, factor reset, disable and delete, with
  `account_in_other_tenants`; per-tenant disable and removal for everyone else. Email has no update path
  today, so there is nothing to guard (M)
- [x] 2.4 Removal revokes that actor's sessions, tokens and keys in the same transaction; removing the last
  membership deletes the account (S)
- [x] 2.5 Verifiers read account and actor disabled state per request, which stops a disabled account in
  every tenant at once; the per-tenant sweep this replaced is not needed (M)
- [ ] 2.6 Tenant deletion deletes accounts left with no actor; `tix doctor` lists orphans (S)
- [ ] 2.7 Snapshot import maps an existing email only for a global admin (S)

## 3. Account log (PR 3, M)

- [x] 3.1 Account log migration, both stores, unscoped allow-list entry and isolation guard (S)
- [x] 3.2 Every account change from PR 2, the second factor, global admin grant and revoke, and invite
  acceptance writes its row in the same transaction (M)
- [x] 3.3 `GetAccountLog` and `GetAccountLogFor`, registry, HTTP, `tix account log`,
  `tix global-admin account-log`, web Account screen, Bulgarian strings (M)

## 4. Shared host, switching, whoami (PR 4, L)

- [ ] 4.1 `whoami` memberships and `account_id` (registry, OpenAPI, client); `account:read` arrived with
  the account log (S)
- [ ] 4.2 `resolveTenant`: credential and `X-Tix-Tenant` on the shared host, refusal on a bound host (M)
- [ ] 4.0 Migration: `sessions.second_factor` and `sessions.method`, written at every session mint (S)
- [ ] 4.3 Same-host switch: `POST /switch`, factor challenge, new session, old one ended, audit (M)
- [ ] 4.4 Web header switcher dialog, `?dialog=tenants` fallback, a tenant on its own domain listed as a
  link to its sign-in page, Bulgarian strings, screenshots (M)

## 5. CLI, TUI, SSH, docs (PR 5, M)

- [ ] 5.1 `tix login --tenant`, `tix tenant ls|use` from memberships, `tix whoami` lists them (M)
- [ ] 5.2 TUI tenant view lists memberships and switches where a credential exists (S)
- [ ] 5.3 `tix ssh key add --all-tenants` (S)
- [ ] 5.4 `docs/tenancy.md`, `docs/web-ui.md`, `docs/api.md`, skill, release note on rollback (S)
- [x] 5.5 Retire `global-admins-and-tenant-bootstrap`'s "Existing users" limitation (S)

## 6. Later phase: cross-host switch (PR 6, M)

- [ ] 6.1 Switch ticket migration; ticket mint and `POST /switch/redeem` page with the confirm button;
  log redaction; the switcher uses it for a tenant on its own domain (M)
