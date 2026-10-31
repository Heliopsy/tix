# The browser speaks the reader's language, and explains its own terms

## Why

Every word the browser interface shows is an English literal in a template, a handler or a script.
There is no catalog, no language negotiation and no way for a reader whose browser asks for another
language to get one. Adding a language today means editing every screen.

Separately, the interface uses words a newcomer has no way to decode from the screen itself: a
claim and its lease, a scope, a state category, `is:recurring`, a lane, a retention period, a sync
run. The only explanations are the `field-info` notes on a few form fields; headings, filters and
figures say nothing.

## What Changes

- A message catalog keyed by the English text (`internal/i18n`). English needs no entries; every
  other language registers one per key. Bulgarian is the first language shipped.
- The language is negotiated from the browser's `Accept-Language` header, unless the settings
  screen's Language row chose one for this browser (`tix_lang`), and written on the root element's
  `lang`. Template sets are parsed per language, the way they already are per time style.
- Templates translate through `t`, `tn` (singular and plural) and `th` (fixed strings with inline
  markup). Flashes, page titles, error headings and hints, state category labels, column, view and
  scheme labels, and pager units translate at the template edge.
- The texts scripts write themselves (copy feedback, the sidebar toggle, the file chooser label) are
  carried translated in a JSON block in the head and read by a new `assets/i18n.js`.
- A `tip` partial: the existing `field-info` disclosure, for a concept rather than a field, with an
  accessible name and an optional pointer into `docs/`. Tips are placed beside the concepts a
  newcomer would not know.
- Tests fail on any template literal that bypasses the catalog, on a key a language lacks or an
  entry nothing uses, on a translation that changes its format verbs or markup, and on a script
  text the head would not carry.

## Out of scope

- Messages composed in Go from parts (the activity feed's sentences, domain error messages from the
  service layer, relative times and durations) stay English: they are shared with the CLI and the
  HTTP API, and translating them needs the catalog to reach `internal/core` and `internal/service`.
- The TUI and the CLI.

## Impact

- `internal/i18n` (new), `internal/web` (render, templates, assets), `internal/web/jstest`.
