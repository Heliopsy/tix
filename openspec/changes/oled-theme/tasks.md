# Tasks

## Scheme

- [x] `oled` and `system-oled` in `web.Themes`, labelled OLED and System, OLED when dark
- [x] OLED token blocks in `010-tokens.css`; system-dark rules exclude both OLED values
- [x] `700-oled.css`: custom project colour shade and no sign-in glow under OLED
- [x] Hover shadow, sign-in glow and tenant accent selectors name OLED
- [x] A custom project colour gets an OLED shade (`--proj-o`)

## Tests

- [x] The switch cycles through six schemes and the cookie and `data-theme` follow
- [x] Every dark token has an OLED value, and the two OLED blocks agree
- [x] OLED tokens clear 4.5:1 for text and 3:1 for the strong line
- [x] Every dark selector list names OLED, every dim exclusion excludes OLED

## Surfaces

- [x] Bulgarian strings
- [x] `shoot.mjs --theme oled|all`, `just ui-check` runs all
- [x] `docs/web-ui.md`, `docs/theming.md`, `docs/testing.md`
