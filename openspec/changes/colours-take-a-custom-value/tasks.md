# Tasks

## 1. Contract

- [x] 1.1 `internal/core/colour.go`: parse `#rrggbb`, relative luminance, contrast, readable shade, nearest palette colour
- [x] 1.2 `internal/core/palette.go`: a project colour is a palette name or a custom hex
- [x] 1.3 `internal/core/theme.go`: a tenant theme of `#rrggbb` resolves to a derived theme
- [x] 1.4 `internal/service/tenant.go`: a tenant may be set to a custom accent

## 2. Browser

- [x] 2.1 `internal/web/colour.go`: per-scheme shades, `projClass`, `projStyle`, the colour field model and `colourField`
- [x] 2.2 `internal/web/templates/partials.html`: the shared `colour-field` control
- [x] 2.3 `projects.html`, `board.html`, `tenant.html`: use the control
- [x] 2.4 `tasks.html`, `partials.html`, `projects.html`, `board.html`: custom colours drawn through `--proj`
- [x] 2.5 `internal/web/assets/app.css`: `proj-custom` per scheme, the control's layout
- [x] 2.6 `internal/web/assets/feel.js`: `tix.hexColour` and the control's wiring
- [x] 2.7 `internal/i18n`: Bulgarian for every new string

## 3. Terminals

- [x] 3.1 `internal/output/color.go`: truecolor or nearest palette swatch
- [x] 3.2 `internal/tui/theme.go`, `internal/tui/project.go`: draw a custom colour, keep it on the edit form

## 4. Guards

- [x] 4.1 core: normalisation, contrast, readable shade, nearest colour, theme resolution
- [x] 4.2 service: custom project colour round trip, tenant custom accent
- [x] 4.3 web: the control on each form, saving a custom colour, the style attribute, scheme backgrounds read from `app.css`
- [x] 4.4 node: `tix.hexColour`
- [x] 4.5 output and tui: custom swatch and style

## 5. Surfaces

- [x] 5.1 `cmd/project.go`, `cmd/tenant.go`: flag help and examples name `#rrggbb`
- [x] 5.2 `internal/tui`: a custom tenant accent from the tenant edit form
- [x] 5.3 `docs/api.md`, `docs/workflows.md`, `docs/configuration.md`, `docs/theming.md`, `docs/tui.md`, `docs/web-ui.md`
