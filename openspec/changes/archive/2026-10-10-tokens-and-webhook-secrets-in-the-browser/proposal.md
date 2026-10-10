# A browser that can kill somebody else's token, a CLI that agrees with it, and a secret the webhook screen actually shows

## Why

Three defects in credential handling, each of which makes a published surface say something that is not
true.

**The browser could not revoke somebody else's token.** `internal/web/admin.go` listed
`ListTokens(ctx, actor.ID)`, so `/admin/tokens` showed the signed-in actor's own tokens and no others. The
command line has taken `--actor` all along, and the service has never restricted `ListTokens` or
`RevokeToken` to the caller's own: `token:admin` reaches the tenant's tokens. The narrowing lived only in
the browser, where it was recorded as a deliberate `Limitation` on the capability registry and written into
an archived capability spec as something the browser deliberately does not do.

It was decided before anybody considered incident response. The case the browser has to serve is a
credential leaking — someone pastes a token into a chat, a laptop goes missing — and that credential is
usually not the reader's own. In exactly that case the only surface a person reaches for under pressure
showed them nothing they could act on, and the answer was "find the identifier, open a terminal, and hope
the CLI is installed on the machine you are holding".

**`tix token create` defaulted to no expiry while the browser form proposed ninety days.** The browser
grew an expiry control with a 90-day proposal; the CLI kept minting immortal tokens when `--expires` was
omitted. One product, two answers to "how long does a credential last by default", and the surface a script
drives was the one handing out the unbounded one.

**`webhooks.html` promised a generated signing secret it never rendered.** The field's own help text said a
generated secret is "shown once, right after you save". `internal/web/webhooks.go` blanked `Secret` on every
listing and no template rendered it anywhere, so an operator who left the field blank got a secret they
could not obtain, from a screen that had just told them they would see it. The service already returns the
generated value exactly once from `PutWebhook`; the browser dropped it on the floor. Meanwhile the token
screen had, in the same release, grown exactly the presentation this needs — a region of its own, a
`.copyline` that wraps, and the generic copy delegate.

## What Changes

- **`/admin/tokens` lists the tenant's tokens to a reader holding `tenant:admin`**, with an Owner column
  naming whose each one is, and offers them the same confirming disclosure to revoke. A reader without that
  scope sees their own and nothing else, exactly as before. The gate is the scope the reader holds, checked
  the way `advancedMode` already checks it; the service's own authorization is unchanged, so the browser is
  never the thing granting authority.
- **Revoking a token that is not yours is audited as `token.revoke_other`**, against the actor who did it,
  where revoking your own stays `token.revoke`. An operator reading the trail after an incident filters on
  the action, not on a payload field, so the distinction belongs in the action.
- **`tix token create` defaults to a 90-day expiry**, matching the browser form. `--expires never` is the
  only way to mint a non-expiring token. `--expires` also accepts a duration such as `30d`, on top of the
  date and timestamp it already took.
- **The HTTP API is left alone.** It has no default of its own: `expires_at` absent means no expiry, and a
  transport that quietly rewrites a field nobody set is worse than one that does what it was told.
- **`/admin/webhooks` renders a generated signing secret once**, through a `secret` partial extracted from
  the token screen so both pages use one copy of the markup, the stylesheet rules and the copy delegate.
  A secret the operator supplied is not shown back to them, and the help text says so in both cases.

## Impact

- **Breaking, for existing CLI callers.** A script that minted a non-expiring token by omitting `--expires`
  now mints a 90-day one. It lands in the same release as token-name uniqueness, which is also breaking.
  `--expires never` restores the old behaviour explicitly.
- `internal/store`: `Tx.GetToken`, implemented by both engines. Additive.
- `internal/service`: `RevokeToken` reads the token before revoking it, to record whose it was.
- `internal/web`: `tokenRow` and the owner column; `oneTimeSecret` and the shared cookie helpers in a new
  `secret.go`; `putWebhook` carries a generated secret to the screen that shows it.
- `internal/web/templates`: a `secret` partial in `partials.html`, used by `tokens.html` and
  `webhooks.html`; no new stylesheet rules, because `.secret` and `.copyline` already exist.
- `internal/capability`: `token.list`'s recorded `Limitation` is rewritten. It is not removed: a reader
  holding `token:admin` and not `tenant:admin` still reaches another actor's tokens from the CLI and not
  from the browser, so the shortfall is real and is now a different one.
- `openspec/changes/archive/2026-09-27-scope-aware-tui-navigation`: the scenario and design note recording
  the old narrowing as deliberate are amended in place rather than left contradicting the code.
- `cmd`: `--expires` gains a default and a duration grammar; `parseExpiry` replaces `parseTime` for it.
- No new dependency. No `internal/core` change.
