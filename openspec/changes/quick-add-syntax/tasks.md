# Tasks

## 1. Spec

- [x] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Parser

- [x] 2.1 `internal/quickadd`: tokenizer, escapes, quotes, sigils and keyed forms
- [x] 2.2 Due date forms: today, tomorrow, weekdays, +Nd, +Nw, absolute dates
- [x] 2.3 `Result.Apply` merges into `core.CreateTaskInput` and refuses contradictions
- [x] 2.4 Table tests, Unicode and Bulgarian titles, emails, `C#`, URLs

## 3. Surfaces

- [x] 3.1 CLI `tix task add --parse`
- [x] 3.2 HTTP `POST /v1/tasks` with `parse: true`
- [x] 3.3 Web one-line box parses, preview route and chips, (i) tip, refusal under the box
- [x] 3.4 TUI new-task prompt parses, previews, keeps the line on refusal

## 4. Docs and tests

- [x] 4.1 features, commands, api, web-ui, tui, agent skill, i18n English and Bulgarian
- [x] 4.2 Surface tests with guards watched failing
- [ ] 4.3 `just check` passes
