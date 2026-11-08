# Multi-tenant follow-ups

## Why

`one-account-many-tenants` shipped one account across several tenants, the shared host and the same-host
switch in 0.46.0 to 0.48.0, and was archived with two tasks still open. Both are worth doing, and both
were designed there, so they move here rather than being lost with the archive:

- **Tenant deletion leaves accounts behind.** An account whose only actor was in a deleted tenant stays in
  `users` with no membership anywhere. It can never sign in, but it still holds an email, a password hash
  and possibly a second factor, and nothing lists it. Accounts left this way before the account to actor
  link exist too.
- **A tenant on its own domain cannot be switched to.** The switcher lists such a tenant as a link to its
  sign-in page, so a person in HO and in `away` signs in twice. The design for a single-use handoff ticket
  was written and reviewed, then deferred as a later phase.

The third open task, 2.2a (a pending invitee is not assignable until accepted), is not carried over: the
owner decided the current behaviour is fine as it is.

## What Changes

- **Tenant deletion deletes accounts left with no actor** in the same step, keeping global admins and
  every account that is still a member of another tenant. Each deletion writes its account log row.
- **`tix doctor` lists accounts with no actor** (orphans from before this change, or from any path that
  leaves one), naming the count and each account's ID and email to a global admin. It deletes nothing.
- **Cross-host switch ticket.** Switching to a tenant served at a domain of its own mints a ticket
  (32 random bytes, stored hashed, 60 seconds, single use, bound to the account, the target tenant and the
  target host) and answers with a page whose one form posts it to the target's `/auth/switch/redeem`. The
  target asks for confirmation, naming the account and tenant, before minting a session.
- **The switcher** offers a Switch button for such a tenant instead of a link to its sign-in page.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `multi-tenancy`: tenant deletion removes accounts it leaves with no actor; `tix doctor` lists orphans.
- `auth`: the cross-host switch ticket.
- `http-api`: `POST /auth/switch` answers with a ticket for another host; `POST /auth/switch/redeem`.
- `web-ui`: a tenant at its own address gets a Switch button.
- `data-model`: the `switch_tickets` table.

## Impact

- One migration (`just migration switch_tickets`): `switch_tickets`, unscoped and on the allow-list in
  `internal/store/sql`, with the isolation guard extended, as `user_totp` and `account_log` are.
- `internal/service`: tenant deletion, `switch.go`, a doctor check for orphans.
- `internal/httpapi`: the redeem route, redacted from the request log as invite routes are.
- `internal/web`: the switcher and the redeem confirmation page, Bulgarian strings.
- `cmd/doctor`: the orphan report.
- Docs: `docs/tenancy.md`, `docs/web-ui.md`, `docs/api.md`.
