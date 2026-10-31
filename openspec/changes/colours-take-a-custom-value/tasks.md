# Tasks

## 1. Contract

- [ ] 1.1 `internal/core/colour.go`: parse `#rrggbb`, relative luminance, contrast, readable shade, nearest palette colour
- [ ] 1.2 `internal/core/palette.go`: a project colour is a palette name or a custom hex
- [ ] 1.3 `internal/core/theme.go`: a tenant theme of `#rrggbb` resolves to a derived theme
- [ ] 1.4 `internal/service/tenant.go`: a tenant may be set to a custom accent

## 2. Browser

- [ ] 2.1 `internal/web/colour.go`: per-scheme shades, `projClass`, `projStyle`, the colour field model and `colourField`
- [ ] 2.2 `internal/web/templates/partials.html`: the shared `colour-field` control
- [ ] 2.3 `projects.html`, `board.html`, `tenant.html`: use the control
- [ ] 2.4 `tasks.html`, `partials.html`, `projects.html`, `board.html`: custom colours drawn through `--proj`
- [ ] 2.5 `internal/web/assets/app.css`: `proj-custom` per scheme, the control's layout
- [ ] 2.6 `internal/web/assets/decide.js`, `feel.js`: `tix.hexColour` and the control's wiring
- [ ] 2.7 `internal/i18n`: Bulgarian for every new string

## 3. Terminals

- [ ] 3.1 `internal/output/color.go`: truecolor or nearest palette swatch
- [ ] 3.2 `internal/tui/theme.go`, `internal/tui/project.go`: draw a custom colour, keep it on the edit form

## 4. Guards

- [ ] 4.1 core: normalisation, contrast, readable shade, nearest colour, theme resolution
- [ ] 4.2 service: custom project colour round trip, tenant custom accent
- [ ] 4.3 web: the control on each form, saving a custom colour, the style attribute, scheme backgrounds read from `app.css`
- [ ] 4.4 node: `tix.hexColour`
- [ ] 4.5 output and tui: custom swatch and style
