# Design

Both parts were designed in `one-account-many-tenants` (archived 2026-11-08); this carries those notes
forward unchanged in substance. Read that change's design.md for the surrounding model.

## Accounts left with no actor

`users` is the account; a person has one actor per tenant, linked by `actors.user_id`. Deleting a tenant
soft-deletes it and its actors stop resolving, so an account whose only actor was there is left with no
membership anywhere.

- Tenant deletion, in the same transaction as the soft delete, deletes each account whose every actor is
  in the deleted tenant. Global admins are kept: they reach every tenant without a membership, and losing
  the last global admin would leave a server nobody can administer. An account still a member of another
  tenant is kept.
- The membership read crosses tenants, so it goes through the unscoped `ListMemberships` reader the
  sole-tenant rule already uses, read before the write as in PR 2 of the parent change: a membership granted
  between the two is granted by a global admin or the person, and neither is the threat.
- Each deleted account writes an `account_log` row (`account.deleted`, the tenant key, by the global admin
  who deleted the tenant).
- Users with no actor already in the database (left by tenant deletion before the account to actor link,
  or by any later bug) are not deleted by a migration. Not deleting them silently keeps migrations pure.
  `tix doctor` lists them; a global admin can delete them with the existing account deletion.

## Cross-host switch

`POST /auth/switch` with the target tenant key, from an authenticated session, as today:

1. Look up the caller's account and its actor and membership in the target. No membership, a disabled
   actor or account: refused as not found (the same answer for a tenant that does not exist).
2. Target requires a second factor on the web and the current session has `second_factor = 0`: the person
   is challenged for a code before anything is minted.
3. Target served on the same host: mint a session, end the old one (shipped).
4. Target served at a verified domain of its own (new): mint a switch ticket and answer with a page whose one
   form POSTs it to `https://TARGET/auth/switch/redeem`. Redeeming re-checks step 1, mints the session there
   and marks the ticket used in the same transaction. The source session is left alone, because it lives on
   another host's cookie.

The ticket is 32 random bytes, stored only as a hash, expires in 60 seconds, is single use, and is bound to
the account, the target tenant and the target host (`audience`). It records whether the source session
passed a second factor, so the target's policy is checked against it.

`switch_tickets`: `id`, `user_id`, `target_tenant_id`, `audience`, `token_hash`, `second_factor`,
`created_at`, `expires_at`, `used_at`. Unscoped and on the allow-list, like `user_totp`, because it is
written in one tenant and redeemed in another. On PostgreSQL it reads through `Unscoped`, named on the
allow-list, and the isolation guard is extended for it.

The redeem page shows "Continue to TENANT as EMAIL" with a button and does not auto-submit. That works
without scripting and makes a forced sign-in (an attacker posting their own ticket into a victim's browser)
visible instead of silent. The ticket never appears in a URL; the redeem route is redacted from the request
log anyway, as invite routes are.

The switch is recorded in the target as `user.login` with method `switch`, never naming the source tenant,
as the same-host switch already is.

## Security review

| Path | Threat | Answer |
| --- | --- | --- |
| switch ticket theft or replay | session in the target | 60 s, single use, hashed, bound to account, tenant and host, POST body only, never logged |
| forced switch (attacker's ticket in a victim's browser) | victim works in attacker's account | redeem page names the account and needs a click |
| ticket minted on host A replayed to host B | session on the wrong host | audience bound; refused |
| tenant deletion deletes a global admin | server left without an administrator | global admins are kept |
| tenant deletion deletes an account still in use | lockout from another tenant | only accounts with no actor in any other tenant are deleted |
