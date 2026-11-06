# Tasks

## 1. Formats and parsing

- [x] 1.1 `output`: `long`, `dotted` and `dayfirst` layouts, `TimeStyle.DateOrder` and `TimeStyle.Date`
- [x] 1.2 `quickadd`: `ParseDueIn`, `ParseIn`, `ParseDate`, `DueExamples`; `ParseDue` and `Parse` stay ISO only
- [x] 1.3 `core.RecurrenceExamples`, each one parsed by a test

## 2. Surfaces

- [x] 2.1 Web: due date fields are text fields read in the reader's order and filled in their format
- [x] 2.2 Web: examples in the due, repeat and quick-add tips; a test parses every example shown
- [x] 2.3 Web: settings offer the new formats with an example each and say how a typed date is read
- [x] 2.4 TUI and `tix task add --parse` read a quick-add date in the configured order
- [x] 2.5 CLI `--due` and `--repeat` help gives examples; `--due` stays ISO and RFC 3339
- [x] 2.6 Bulgarian catalog entries
- [x] 2.7 Web: calendar button beside each due text field (`assets/datepick.js`), placeholder with examples

## 3. Docs

- [x] 3.1 `docs/configuration.md`, `docs/web-ui.md`, `docs/tui.md`
- [x] 3.2 `skills/tix`
