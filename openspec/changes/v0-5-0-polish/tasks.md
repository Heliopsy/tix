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

## Release

- [ ] `just ci` green
- [ ] docs and ROADMAP updated
- [ ] archive this change
