# List wrapping and a settings navigation that follows the reader

## Why

On the projects screen a project's name broke inside a word, sometimes leaving its last letter alone on
a line. The row's Pin, Tasks, Fields and Manage links are always laid out, invisible until hover on a
pointer screen, and sit on the same line as the name. In the two-column grid from 1200px, and at any
width in Bulgarian where the links are longer, they took most of the row. The name's
`overflow-wrap: anywhere` then let its column shrink to one character, so "Infrastructure" came out as
"Infrastructur" over "e". Table cells and the record and people lists carry the same rule, and a short
word such as "Никога" on the token list broke the same way.

The settings screen marks a section in its navigation only after a jump to it (`:target`), so the mark
stays on the section last clicked while the reader scrolls through the others.

## What Changes

- A project row keeps at least a readable width for its name and description and moves its links onto a
  line of their own when they do not fit beside it, instead of squeezing the name.
- Names, keys and words in list rows and table cells break only at word boundaries. Only a token with no
  break opportunity that is wider than its cell is broken inside, and identifiers (`code`, monospaced
  values, links) keep breaking anywhere so a table never forces the page wider.
- A clamped project description carries its full text as a tooltip.
- The screenshot layout check reports a word split in a list row or table cell that leaves a single
  character on a line, or that splits a word short enough to fit; the existing spill check already
  reports content overflowing a cell. The
  hostile seed adds short and single-word project names, a 1280px `desktop` viewport joins `ui-rows`,
  and `--locale` captures the screens in another language.
- A small script, `assets/tocspy.js`, follows the reader's scroll on any `nav[data-toc]`, marking the link
  for the section in view with `aria-current="true"`. A click scrolls smoothly unless reduced motion is
  asked for, updates the address fragment and moves focus to the section. Without scripting the plain
  anchors and the `:target` mark work as before. The settings screen is the only one with such a
  navigation today.

## Capabilities

### Modified Capabilities

- `web-ui`: list and table text wrapping, the project row's tools, and the settings navigation's current
  section.

## Impact

`internal/web/assets/css` (220-projects, 080-tables, 130-quick-add-and-filters, 210-page-header-and-create-forms,
260-users, 380-record-list, 360-settings, and 710-list-wrapping for the Bulgarian priority cell),
`internal/web/assets/tocspy.js`, `internal/web/templates/projects.html`, `settings.html`, `layout.html`,
`actors.html`, `tokens.html`, `sync.html`, `syncruns.html`,
`scripts/screenshots/shoot.mjs`, `justfile`, `docs/web-ui.md`, `docs/testing.md`.
