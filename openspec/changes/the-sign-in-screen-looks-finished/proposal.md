# The sign-in screen looks finished

## Why

`/login` is the only tix screen a visitor sees before they are anybody, and it
was the generic shell-less layout with nothing added: a 360px column, a bare
`h1`, a default `.card` holding two inputs, and an accent-filled button sized to
its own label. No brand mark, no composition, nothing that says which product
this is. The sign-in screen is the whole first impression of an install, and it
read as a form that had not been designed yet.

It also carried the only colour in tix with nowhere to show itself. A tenant
sets an accent with `tix theme`, and the screen that introduces the tenant used
it on one button.

## What Changes

**The sign-in screen becomes a composed page, and every colour in it stays a
token.** The layout is a single centred column: brand lockup, card, secondary
action, with a large soft accent glow behind the card.

- **The glow is the one distinctive element.** A 749px circle centred on the
  card, in `--accent`, softened by its own gradient stops rather than by a blur
  filter. It is the tenant's colour used as the page's atmosphere instead of as
  a single button fill, and it costs one pseudo-element.

- **The card is the composition, not the page.** Brand lockup and wordmark above
  it with 50px of air, ~446px of content inside 32px of padding, a 16px radius
  and a soft lift. Inside: a centred 18px heading, a small bold label above each
  input, 6px input corners, and a full-width 40px submit.

- **The submit is painted `--ink` on `--surface`, not `--accent`.** Both tokens
  flip with the scheme, so the button is the inverse of the card it sits in
  rather than a fixed black, and the accent is left to the glow. One strong mark
  per screen.

- **The "can't sign in?" disclosure becomes a muted underlined line of text**
  below the card instead of a bordered panel competing with the card above it.
  It stays a `<details>`, so it still opens with scripting off, and its contents
  are unchanged.

**Nothing about the auth path moves.** The CSRF field, the refusal rendering,
the `?next=` redirect and every handler in `internal/web` are untouched. This is
a stylesheet and one template.

## Impact

- `internal/web/templates/login.html`: a brand lockup above the card, and the
  card gains an `authcard` class so the new rules can hang off it.
- `internal/web/assets/app.css`: one appended block, delimited by a comment.
  Appended rather than merged into the existing `.auth` rules because the sheet
  is edited by more than one change at a time and a hunk in its middle costs a
  conflict.
- `internal/web/loginlook_test.go`: new.

**What does not change.** The screen is reachable, public and scriptless exactly
as before, the disclosure still carries the same three commands and still never
reacts to what was typed, and the refusal screen that shares the shell-less
layout keeps its own measure: every new rule is scoped to `.authcard`, which
`error.html` does not carry.
