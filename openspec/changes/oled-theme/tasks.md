# Tasks

## Scheme

- [ ] `oled` and `system-oled` in `web.Themes`, labelled OLED and System, OLED when dark
- [ ] OLED token blocks in `010-tokens.css`; system-dark rules exclude both OLED values
- [ ] `700-oled.css`: custom project colour shade and no sign-in glow under OLED
- [ ] Hover shadow, sign-in glow and tenant accent selectors name OLED
- [ ] A custom project colour gets an OLED shade (`--proj-o`)

## Tests

- [ ] The switch cycles through six schemes and the cookie and `data-theme` follow
- [ ] Every dark token has an OLED value, and the two OLED blocks agree
- [ ] OLED tokens clear 4.5:1 for text and 3:1 for the strong line
- [ ] Every dark selector list names OLED, every dim exclusion excludes OLED

## Surfaces

- [ ] Bulgarian strings
- [ ] `shoot.mjs --theme oled|all`, `just ui-check` runs all
- [ ] `docs/web-ui.md`, `docs/theming.md`, `docs/testing.md`
