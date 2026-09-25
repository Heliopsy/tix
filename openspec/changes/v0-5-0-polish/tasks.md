# Tasks

## Web

- [x] `web`: scope the completion animation to the row that was acted on
- [x] test: a rendered list of completed tasks animates nothing
- [x] test: completing a task animates exactly one mark

## Theming

- [x] `docs/theming.md`: `tix tenant edit`, not `tix tenant set`
- [x] `docs/theming.md`: an unnamed theme resolves to the default, not a derived colour
- [x] `cmd/tenant.go`: the `--theme` flag help says the same
- [x] spec: record the default-theme behaviour as a modified requirement

## External sync

- [x] `service/sync.go`: a failed full refresh leaves the stored watermark alone
- [x] `service/sync.go`: the audit entry carries the watermark the run reached
- [x] `core`: an upstream-failure error kind, mapped to an exit code and an HTTP status
- [x] `service/sync.go`: `fail` reports upstream, not internal
- [x] `output`: render `SyncResult` in table, json and yaml
- [x] tests for each of the four

## Demo seed

- [x] `internal/demo` + `cmd/demo.go`: history replayed through the service on a steppable clock
- [x] descriptions, tags, due dates, assignees and custom fields
- [ ] screenshots regenerate from the new seed

## Durations

- [x] `core.Duration.Human`, shared by the browser, command line and terminal
- [x] guard: the statistics table prints no machine-rendered duration

## Web, found while reviewing the demo

- [x] the tick mark clears itself; `.is-target` keeps its deep-link meaning
- [x] a way back to the listing a task was opened from
- [x] the assignee field shows a handle, not an identifier
- [x] a Clear control on the filter, and the project filter says when it overrides visibility
- [x] lease badges: held, and claim expired
- [x] the date says what it is; the reference reads as an identifier
- [x] Columns and Projects merged into one View panel
- [x] the status pill is a control, offering the workflow's own moves
- [x] assets carry a cache validator so an upgrade cannot serve a stale stylesheet
- [x] the status panel is a popover, so a listing's overflow cannot clip it
- [x] multi-step moves, route shown first, one audit entry per step
- [x] per-browser date format and timezone
- [x] `ListActors` end to end, and an assignee picker that suggests without constraining

## Release

- [ ] `just ci` green
- [ ] docs and ROADMAP updated
- [ ] archive this change
