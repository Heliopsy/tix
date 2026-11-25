# Design

## Where the theme is resolved

`core.ThemeRegistry.Resolve` already holds the rules (custom `#rrggbb` accent, configured name, built-in
name, default fallback) and the web and the terminal both call it. The service gets the same registry
through `service.WithThemes`, which existed but was never passed in production; `internal/connect` now
builds it from configuration next to the other service options. whoami resolves the caller's tenant
through it inside the read it already makes for the tenant key.

The registry already fills the dark pair from the light one for a configured theme that names none, and a
custom accent derives both, so every resolved theme carries four colours and whoami answers exactly what the
browser paints. A client never repeats either rule.

## Loading configured themes

`themes` is a map by name, so it cannot go through the flat key table, and the file loader refused it as
`themes.NAME.accent`, an unknown key. It is now decoded on its own, as `contexts` is, with unknown fields
refused, and `config.Theme` gains `accent_dark` and `accent_soft_dark`, which docs/theming.md already
documented. A discovered per-directory file's themes overlay the global file's by name. `SaveChanges`
writes the file's own themes back, so `tix ctx use` and friends do not strip them.

## Shape

`theme` is a `core.Theme` as `tix theme ls -o json` prints it: `name` (the theme name, or the `#rrggbb` a
custom accent was given as), `accent`, `accent_soft`, `accent_dark`, `accent_soft_dark` and `built_in`.
Reusing the type keeps one documented palette shape. State and priority colours stay out, as on every
surface.

## Who may read it

Any credential in the tenant. The accent and the tenant's key and name are what the browser shows every
member on every page, so they are not secrets to a token minted there. The rest of the tenant record
(second factor requirements, retention, limits) stays behind `tenant:admin`, and memberships keep their
`account:read` rule.

## Surfaces

- HTTP and MCP: the existing `GET /whoami`, regenerated with `just openapi`.
- CLI: `tix whoami` prints a `theme` line and `theme` in JSON and YAML.
- Web and TUI: unchanged, they already render the resolved theme.
