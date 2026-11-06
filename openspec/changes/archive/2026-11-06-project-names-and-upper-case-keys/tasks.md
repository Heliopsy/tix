## 1. Display helpers

- [x] 1.1 `output.ShowKey` and `output.ShowRef`, with tests
- [x] 1.2 Template helpers `key`, `ref`, `joinKeys` and `projLabel`

## 2. Web

- [x] 2.1 `tix_project_label` cookie, `/projectlabel` route, Settings row with a tip, docs row
- [x] 2.2 Every project picker uses the reader's label style with the key as its value
- [x] 2.3 Refs and keys drawn upper case in templates, titles, toasts, activity, palette and recent
- [x] 2.4 Bulgarian strings
- [x] 2.5 Tests: every picker under every style, the setting, a long name, upper-case rendering,
      case-insensitive addresses and filters

## 3. CLI, TUI, calendar

- [x] 3.1 Table and text output draw refs and keys upper case; JSON, YAML, NDJSON and CSV unchanged
- [x] 3.2 TUI draws refs and keys upper case
- [x] 3.3 Calendar feed SUMMARY upper case, UID and URL unchanged
- [x] 3.4 Case-insensitive refs, keys and filters on the CLI, with tests

## 4. Service and API

- [x] 4.1 Copy, statistics, sync and new-task project lookups go through `lookupProject`
- [x] 4.2 API test: any case in, stored value out

## 5. Docs

- [x] 5.1 docs, README and skills show upper case where they show rendered output
