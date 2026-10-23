# Design

## The property being delivered

The sign-in screen presents the tenant, in the tenant's own colour, in every
scheme tix ships, with no JavaScript. Everything below is in service of the
second half of that sentence: the visual change is cheap, and the ways it could
have broken theming are not.

## Every colour is a token, and a test says so

A login page is the easiest place in a product to hard-code a colour. It is
drawn once, it is looked at in one scheme, and the result looks right to
whoever wrote it. tix has two reasons that is not available here: `tix theme`
lets a tenant pick its accent, and the stylesheet carries `dark` and `dim`
schemes plus a `prefers-color-scheme` mirror of `dark`.

So the block resolves `--accent`, `--ink`, `--surface`, `--muted` and `--line`
and nothing else, and `TestSignInRedesignHardCodesNoColour` reads the block and
fails on any hex literal in it. The one exemption is the card's shadow, which is
an `rgba` lift rather than a themed colour and is already how `--shadow` is
written.

The accent is reached through `color-mix(in srgb, var(--accent) N%, transparent)`
rather than through a second set of rgba tokens. A tenant accent arrives as a
hex string in a `<style>` block the layout emits, so there is no channel
decomposition to mix against; `color-mix` is the only way to take an alpha of a
colour that is not known until the page is served.

## Why the glow is a gradient and not a blur

`filter: blur()` on a 749px box is a full-size offscreen paint on every frame
the compositor touches it, for a shape that is a radial gradient by
construction. The stops do the softening: opaque-ish at the centre, 40% at half
the radius, transparent at 68%. The visible edge therefore sits at 68% of the
radius, which is what makes the circle look soft-edged without any filter at
all.

## The glow had to be capped to the viewport

A 749px absolutely positioned child of a 510px column overhangs a phone
viewport by roughly 180px each side, and that overhang is scrollable width.
`body { overflow-x: hidden }` does not stop it: measured in Chrome at a 390px
viewport, the page scrolled 180px sideways onto empty background, which is the
"no horizontal page scrolling" requirement failing on the first screen of the
product.

The fix is `--auth-glow: min(749px, calc(100vw - var(--gutter) * 2))`, used for
both dimensions so the shape stays a circle. The gutter subtraction, rather than
a bare `100vw`, is what keeps a vertical scrollbar from reintroducing the
overflow it was meant to remove.

## Why the dark schemes get their own alphas

The same stops in `dark` and `dim` do not read as the same design. Those schemes
take the bright end of the colour ramp — the default accent is `#2dd4bf` in
`dark` against a `#14171b` page — so a 95% centre stop is a lamp pointed at the
reader rather than a wash behind a card. The dark schemes take 34% and 14%.

This is three copies of one gradient: the explicit `dark` and `dim` selectors,
and the `prefers-color-scheme` mirror. That mirror is not optional. The
stylesheet already records why: scheme rules written as
`:root:not([data-theme="light"])` outrank a plain `:root`, and a block that
redefines only some tokens leaves the rest at their light-mode values in the
scheme a browser defaults to.

## Scoped to `.authcard`, not to `.auth`

`.auth` is the shell-less layout, and `error.html` renders in it too. The first
version of this put the measure, the heading size and the disclosure styling on
`.auth` directly, and the refusal screen silently picked up an 18px centred
heading and a 510px measure it was never designed for.

Every rule now hangs off `.authcard`, a class only the sign-in card carries, and
the two that genuinely belong to the column use `.auth:has(.authcard)`. `:has`
is already load-bearing in this sheet (`label:has(> input[type="checkbox"])`),
so this adds no new support floor.

## What was deliberately not done

- **The form is not touched.** Same action, same field names, same CSRF
  template, same `required` attributes. A visual change that edited the form
  would have to argue about the auth path, and it has nothing to say about it.
- **The disclosure is still a `<details>`.** A `<dialog>` or a scripted toggle
  would read better and would not open with scripting off, which the product
  rule forbids.
- **No font is downloaded and no framework is added.** The page is the system
  font stack and one pseudo-element.
