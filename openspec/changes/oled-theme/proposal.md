# An OLED colour scheme

## Why

The dark scheme paints its page `#14171b` and its surfaces a step lighter. On an OLED screen every one
of those pixels is lit, so a reader who chose dark to save power or to sit in a dark room still gets a
grey glow, and the sign-in screen's accent glow is a lamp. The owner asked for a true black scheme beside
light and dark.

## What Changes

- A fifth scheme, **OLED** (`tix_theme=oled`): a `#000` page, near-black surfaces that read as raised
  through their borders rather than a grey fill, slightly softer ink, and the dark scheme's accents and
  project palette, which clear 4.5:1 on black with room to spare.
- A sixth position, **System, OLED when dark** (`tix_theme=system-oled`): light while the device asks
  for light, OLED while it asks for dark. Plain System keeps mapping to light and dark and stays the
  default.
- The sidebar switch cycles System, Light, Dark, OLED, Low contrast, System OLED when dark. The settings
  screen still names the scheme in force and points at the switch.
- A tenant's dark accent and a custom project colour's shade apply under OLED as under dark, and the
  sign-in screen drops its glow there.
- The screenshot harness takes `--theme oled` and `--theme all`, and `just ui-check` runs all three.

## Capabilities

### Modified Capabilities

- `web-ui`: two more colour schemes on the sidebar switch, held to the same contrast floor.

## Impact

`internal/web` (the scheme list, labels, custom colour shades), `internal/web/assets/css/010-tokens.css`
and a new `700-oled.css`, the inline accent block in `layout.html`, Bulgarian strings,
`scripts/screenshots/shoot.mjs`, `justfile`, `docs/web-ui.md`, `docs/theming.md`, `docs/testing.md`.
The terminal interface has no light or dark scheme, only the tenant accent and `NO_COLOR`, so it is
unchanged: OLED is a browser scheme.
