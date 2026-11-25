# Design: rotate an API token

## Signature

`RotateToken(ctx, id string, in RotateTokenInput) (*IssuedToken, error)`, the same shape as
`ReissueInvite`: the token is addressed by the path and the body is optional. `id` is a token identifier or
a live token's name; a live name is unique in the tenant, so it resolves to at most one token.
`RotateTokenInput` carries `grace` and `actor_id`. `actor_id`, when set, has to be the actor the token
belongs to, so a script naming the wrong agent is told the token is missing instead of rotating someone
else's.

## Lifetime

The new token keeps the old token's lifetime counted from now: `expires_at = now + (old.expires_at -
old.created_at)`, and no expiry when the old one had none.

Counting the remaining time instead would make each rotation shorten the credential, and rotating a token
on the day before it expires would hand out a one-day token. Rotation is the routine way to replace a
credential, so it should behave like issuing it again with the same form. A token that never expires stays
that way, because turning it into an expiring one is a policy change that belongs to issuing a new token.

## Grace

`grace` is a duration from `0` to `24h`; `0` retires the old token at once. The old token's `revoked_at` is
set to `now + grace`, or to its own expiry if that comes first, inside the rotation transaction.

- No background job, and nothing to run at the end of the window: authentication already compares the
  token against the clock.
- `APIToken.Active(now)` treats a `revoked_at` that is still in the future as live. `Retiring(now)` says the
  token is inside its window, which the listing screens show as "retiring".
- The live-name unique index keys on `revoked_at IS NULL`, so the old token releases its name the moment
  it is rotated and the new token can carry it while the old one still works.
- `RevokeToken` during the window sets `revoked_at` to now, cutting the window short.
- `RevokeActorTokens` (disabling a user) matched `revoked_at IS NULL` and would have left a retiring token
  working until its window ran out. It now also ends tokens whose `revoked_at` is in the future.
- A token that is already retiring, revoked or expired cannot be rotated: there is nothing live to replace,
  and a second rotation of a retiring token would mint a second live token for the same name.

The cap is 24 hours. Grace exists so a running agent can pick up its new value; a day covers a deploy cycle,
and a longer window turns a rotation after a leak into a promise that the leaked value keeps working.

## Authorization

`internal/authz` decides, through a new `Policy.CanRotateToken(actor, owner, project)`:

- The token's own actor may rotate it. A project-pinned caller may rotate only a token pinned to its own
  project, so a pinned token cannot use a sibling to mint itself an unpinned credential.
- Anybody else needs `token:admin`, the same as revoking another actor's token.

The service then applies the scope ceiling the issuing path already applies, reused rather than copied:
every scope of the old token except `global:admin` and `account:read` must be held by the rotator;
`global:admin` needs the rotator to be a global admin now and the token's actor to still be one;
`account:read` needs the rotator to hold it when the rotator is a token, and the token's actor to be the
rotator's own person. A token rotating itself passes trivially, because it holds exactly its own scopes.

Because the scope the service enforces depends on whose token it is, the registry declares no scope for
`token.rotate` and records why in the authority test's list of operations whose scope varies by input.

## Audit and event

One transaction writes the new row, moves the old row's `revoked_at`, writes one audit entry and one outbox
event `token.rotated`. The action is `token.rotate` for the rotator's own token and `token.rotate_other` for
another actor's, for the same reason revocation distinguishes them. The entry names the old and new token
ids, the token's actor, the name, the scopes and the time the old token stops; no secret.

## Surfaces

- CLI: `tix token rotate ID|NAME [--grace D] [--actor A] [--dry-run]`, printing the issued token like
  `tix token create`, so `-o json` gives the value. SSH binds it like the other token commands and writes it
  to the session's stdout.
- HTTP: `POST /api/v1/tokens/{id}/rotate` with an optional body, answering `201` and the issued token.
- Web: a Rotate button beside Revoke opens a dialog naming the token, whose it is, and a grace select. The
  value is shown once through the same one-time cookie the issue form uses.
- TUI: `R` on a live token in the account view opens a grace form, then a confirmation naming the token,
  then the one-time reveal. The terminal lists only the reader's own tokens, so it rotates only those.
