# Design

## Where a scheme is chosen

The colour scheme is a per-browser cookie, `tix_theme`, set only by the sidebar switch and rendered as
`data-theme` on `<html>`. It is not kept with the account or the tenant, so there is no global, tenant,
user hierarchy to extend: OLED is one more value of the cookie. No link or kiosk parameter carries a
scheme, and the terminal interface has no scheme concept, so neither changes. The tenant accent
(including the `(derived)` label `tix tenant ls` prints for a tenant that names no theme) is a separate
axis and works under OLED unchanged.

## Tokens

OLED is its own token block, `:root[data-theme="oled"]`, in `010-tokens.css` beside dark and dim, and the
same declarations again under `@media (prefers-color-scheme: dark)` for `system-oled`. A test reads both
blocks out of the bundled stylesheet and fails when the dark block declares a token OLED does not, or
when the two OLED blocks differ, so a token added to dark later cannot leave OLED drawing the light value.

- `--bg` is `#000`. `--surface` is `#0a0b0d`, close enough to black that a card is not a grey slab, so
  elevation is carried by the lines: `--line` `#272c33` and `--line-strong` `#5a636f`, which clears
  3:1 on the page and the surface, because without a fill the border is the only edge a field has.
- `--ink` is `#dde1e6` rather than dark's `#e7eaee`: still over 15:1, and less halation of white text
  on black.
- Accents, state colours and the project palette are dark's. They were tuned for 4.5:1 on `#1b1f24`,
  so on `#0a0b0d` they clear it by more. The soft tints are darker than dark's so a selected row does
  not light a band.
- `--on-accent` and `--on-ok` stay dark's, which clear 4.5:1 on the accents they sit on.

A test computes the WCAG contrast of every OLED text token on the page and the surface, the accents'
foregrounds on them, and the ink on every soft tint, so a retune that drops one under 4.5:1 fails.

## Selectors

Three kinds of rule decide a scheme, and each learns OLED:

- the explicit `:root[data-theme="dark"]` lists (hover shadow, sign-in glow, tenant accent) add
  `:root[data-theme="oled"]`;
- the system-dark rules written `:root:not([data-theme="light"]):not([data-theme="dim"])` add
  `:not([data-theme="oled"]):not([data-theme="system-oled"])`, since they outrank a plain
  `:root[data-theme="oled"]` on specificity and would otherwise paint OLED in dark's greys;
- the tenant accent's system-dark rule already matches OLED and `system-oled` under a dark device, which
  is what both want, so it is left as it is.

A test walks every selector in the stylesheet and fails on a dark list without OLED, or a dim exclusion
without the two OLED exclusions.

## System, OLED when dark

Offered, as its own position on the switch rather than as a second setting. The reader who wants OLED is
mostly on a phone that switches to dark at night; pinning OLED would paint the page black at noon too,
and a separate "prefer OLED for dark" toggle would be a second cookie that only means anything under one
value of the first. Its cost is one more press on the cycle. Plain System still maps to light and dark,
and stays the default for a browser that never chose.

## Large bright areas

The sign-in card's accent glow is dropped under OLED. Primary buttons keep their accent fill: they are
small, and a black button with an accent outline reads as secondary.

## Screenshots

`shoot.mjs --theme` takes `light`, `dark`, `oled`, `both` (light and dark, as before) or `all`, and an
OLED run sets the device to dark so `system-oled` and plain OLED draw the same. `just ui-check` moves to
`all`. The strict checks are about layout, which OLED does not change, but a scheme nobody shoots is a
scheme nobody looks at.
