# Colours take a custom value

## Why

A project's colour could only be one of eight palette names, and the tenant accent one of the
shipped or configured themes. Someone whose team already has a colour for a project, or whose
company has a brand colour, had no way to use it: the palette is a good starting point and a poor
ceiling. Every colour control in the browser should offer the predefined choices and a custom one.

## What Changes

- A project colour is either a palette name, as before, or a custom `#rrggbb` value. Both live in
  the existing `projects.color` column, so there is no migration and every stored value stays
  valid. Anything else is still refused by the service on every access path.
- A tenant may name a custom `#rrggbb` accent instead of a theme name. The soft tint and the dark
  scheme pair are derived from it.
- Every colour control in the web interface (new project, project settings, tenant theme) is one
  shared control: the predefined swatches, plus a custom colour with a native colour input, a hex
  text field and a live preview, with an (i) tip. It works without scripting through the hex field.
- A custom project colour renders through the same `--proj` custom property the palette classes set,
  with a shade derived per scheme (light, low contrast, dark) that clears 4.5:1 against that
  scheme's page and surface, which is the floor the tuned palette already meets.
- The terminal interface draws a custom colour as itself, downsampled to the terminal's depth. The
  CLI table draws it in truecolor when `COLORTERM` says so and as the nearest palette colour
  otherwise.

## Capabilities

### Modified Capabilities

- `data-model`: a project colour admits a custom hex value.
- `web-ui`: colour controls offer the palette and a custom colour; custom colours stay legible.
- `theming`: a tenant may name a custom accent.
- `cli`: a custom project colour on a terminal.

## Impact

- `internal/core`: colour parsing, contrast and derivation; palette validation; theme resolution.
- `internal/service`: tenant theme validation accepts a hex accent.
- `internal/web`: shared colour control partial, per-scheme derivation, templates, CSS, JS, i18n.
- `internal/output`, `internal/tui`: rendering a custom colour.
- No schema change and no API shape change: `color` and `theme` stay strings.
