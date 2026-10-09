# A token name that identifies one token, and an expiry a browser can set

## Why

Five defects on `/admin/tokens`, four of them about a credential rather than about a layout.

**The one-time secret was styled as a confirmation.** The issued value went into a `.flash`: the fixed
420-pixel toast in the corner of the page that says "task saved", inside a `<pre>` that overflowed it. The
single string nobody can ever fetch again was presented as the most transient element the interface has,
cropped, and with no way to copy it.

**A submission with no scope answered with the error screen.** The scope control carried no `required`, so
an empty selection posted, the service refused it, and the reader landed on a page carrying the message
and none of their work. Nothing in this interface re-rendered a form with its values and a refusal, so
there was no pattern to follow.

**Two live tokens could share a name.** Nothing checked. The name is the only thing on the revocation
control that tells one token from another — the identifier beside it is a ULID and the scope lists are
usually identical across the tokens an operator is choosing between — so two tokens called `ci` made
"revoke the one that leaked" a guess. That is a security defect, not an untidy listing.

**There was no Created column.** The data was on the row. `tix token ls` has shown CREATED all along, so
the browser was the surface missing it.

**There was an Expires column and no way to set an expiry.** `core.CreateTokenInput.ExpiresAt` exists, the
service honours it and `tix token create --expires` sets it; the browser form carried `name` and `scopes`
and nothing else, so every token it minted was immortal and the column was permanently blank. A column
that can only ever be empty claims to show something the screen cannot produce.

Three more, found while reading the rest of the page: a revoked token rendered as a row identical to a live
one, under a Revoke control that would do nothing; an expired token likewise; and revocation, which is
irreversible and ends a credential something is holding right now, was a single unguarded click, unlike
every other destructive control in this interface.

## What Changes

- **A live token's name is unique within its tenant.** Enforced in `internal/service` so the command line
  and the HTTP API are held to it as well as the browser, with a partial unique index behind it because two
  creations racing under read committed would both see the name free. Revoked tokens keep their names and
  release them, so rotation stays "revoke `ci`, mint `ci`".
- **An upgrade renames pre-existing duplicates rather than revoking them.** Revoking would stop an agent
  that is working right now in order to tidy a listing.
- **The browser form sets an expiry, and proposes one.** Whole-day choices rather than an instant, with
  "Never expires" as an explicit option rather than the consequence of a form that could not ask. The
  proposed default is 90 days: until this control existed every token the browser minted was immortal by
  omission, so no reader's choice is being overridden.
- **A refusal the reader can act on re-renders the form in place**, with the message beside the control it
  is about and every value preserved. It is the response to the ordinary POST, so it is what a browser
  with scripting off gets; `required` on the control is a convenience on top of it, never the check.
- **Each refusal names the field it is about**, through `core.DetailField`, so the attribution lives with
  the rule rather than in message-matching at the presentation layer.
- **The listing gains Created and State columns**, says "Never" where a token has no expiry, hides the
  revoke control on a revoked token, and puts revocation behind a disclosure that names what it ends.
- **The issued value gets its own region**, in the `.copyline` affordance the sign-in help already uses:
  it wraps rather than overflowing, it is selectable in full, and its copy button is the one generic
  `data-copy-target` handler.

## Impact

- `internal/core`: `DetailField`, `FieldOf`, and a field detail on each `CreateTokenInput` refusal.
  Additive.
- `internal/store`: `Tx.TokenNameInUse`, implemented by both engines.
- `internal/store/migrations`: `0010_token_name_unique.sql`, which renames existing duplicates and adds a
  unique index over `(tenant_id, name)` for rows that are not revoked.
- `internal/service`: `CreateToken` refuses a duplicate name as a conflict naming the `name` field.
- `internal/web`: `fieldErrors` and the `field-error` partial, reusable by any screen; the token form's
  expiry control; the token listing's Created and State columns; `.secret`, `.field-error` and the token
  state badges in `app.css`.
- Behaviour change for existing callers: a `CreateToken` naming a live token's name now fails where it
  previously succeeded. This affects the CLI and the HTTP API, not only the browser, which is the point.
- No CLI surface change: `tix token create` already has `--expires`, and `tix token ls` already shows
  CREATED.
