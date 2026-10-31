# Design

## One column, two shapes

`projects.color` is already `TEXT NOT NULL DEFAULT ''`. A custom colour is stored there as a
lowercase `#rrggbb`. A palette name never starts with `#`, so the two shapes cannot be confused, and
every value written before this change is still one of them. No migration, nothing for SQLite and
PostgreSQL to disagree about, and the API keeps `"color": "<string>"`. The tenant theme column works
the same way: a theme name, or `#rrggbb`.

`core.NormalizeProjectColor` accepts a palette name in any case, or `#` and six hex digits in any
case, and lowercases both. Short forms (`#fff`), names (`rebeccapurple`) and `rgb()` are refused,
because the value is put into a `style` attribute and read by a terminal, and one shape is the
easiest thing to keep out of both.

## Rendering on the web

The palette classes set `--proj` per scheme from tuned tokens. A custom colour cannot ship a tuned
token, so the server derives one per scheme instead: the chosen colour, moved towards black on the
light scheme or white on the dark ones just far enough to clear 4.5:1 against that scheme's page and
surface. A colour that already clears is left exactly as chosen. The element carries
`class="proj-custom"` and `style="--proj-l: …; --proj-m: …; --proj-d: …"`, and the stylesheet picks one
of the three into `--proj` per scheme, so every existing rule that reads `--proj` (stripe, wash, key,
swatch, card border) draws a custom colour with no change of its own.

Only `projStyle` writes into the attribute, and it writes only colours it formatted itself from a
parsed value, so nothing a user typed reaches the stylesheet.

The scheme backgrounds the derivation targets are constants in `internal/web`; a test reads them out
of `app.css`, so the two cannot drift.

Text on a tinted row was checked: `--ink` on the strongest point of the wash stays far above 7:1 in
every scheme for any custom colour. `--muted` on the strongest point of the wash is below 4.5:1 for
the darkest colours on the light scheme, as it already is for several palette colours; the wash fades
out within the first quarter of a row, where no muted text sits. Not changed here.

## Tenant accent

`ThemeRegistry.Resolve` turns a tenant theme of `#rrggbb` into a theme: the accent derived for the
light scheme (clears 4.5:1 on white, which is also white text on the accent), a soft tint of 10% on
white, an accent derived for the dark schemes against the dark page, and a soft tint of 22% on the dark
page. `UpdateTenant` accepts a valid hex as well as a name that resolves.

## One control

`colour-field` in `partials.html` takes a field name, the predefined options (value, label, swatch
CSS), the current value and whether "none" is offered. It renders a radio group of swatches, then a
"Custom" radio with an `<input type="color">` and a `#rrggbb` text field and a preview chip. The radio
group posts `name`; the custom radio posts `#custom`, which no palette colour or theme is named, and the
text field posts `name_custom`, which the handler folds back with `colourField`. Without scripting the
text field alone works. `feel.js` keeps the colour input, the text field and the preview in step and
ticks the custom radio when either is touched; its hex parsing is `tix.hexColour`, tested in the node
suite.

The terminal interface's edit forms offer `custom` after the palette (and after the themes on the
tenant form), which hands over to the single-line input for a `#rrggbb`, seeded with the current custom
value.

## Terminals

The TUI hands lipgloss the hex and the theme's colour profile downsamples it, which is truecolor
where the terminal declared it and the nearest 256 or 16 colour otherwise. The CLI painter has no
profile, so it looks at `COLORTERM`: `truecolor` or `24bit` gets `38;2;r;g;b`, anything else gets the
SGR of the nearest palette colour by the palette's light-scheme shades.
