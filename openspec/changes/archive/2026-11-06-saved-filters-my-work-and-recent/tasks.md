# Tasks

## 1. Model and storage

- [x] 1.1 `internal/core/savedfilter.go`: `SavedFilter`, inputs, `SavedFilterService`, `MeRef`
- [x] 1.2 `internal/core/view.go`: the `mine` view and `DueBucketOf`
- [x] 1.3 Migration 0021 and the store methods on both engines
- [x] 1.4 `internal/service/savedfilter.go`: six methods, one transaction each with audit and event;
      `bindMe` in task listings
- [x] 1.5 `internal/authz`: `filter.save`, `filter.manage`; registry: six entries

## 2. Surfaces

- [x] 2.1 `internal/wire`, `internal/httpapi`, `internal/client`: the filter routes
- [x] 2.2 `cmd/filter.go`; `tix task ls --saved` and `--mine`
- [x] 2.3 `internal/web`: Save in the filter bar, the sidebar list and its menus, My work, the recent list
- [x] 2.4 `internal/web/assets`: `recent.js`, icons, styles, rail; i18n with Bulgarian
- [x] 2.5 `internal/tui`: saved filter picker, save the current filter, `me` in the in-memory filter

## 3. Documentation

- [x] 3.1 `docs/api.md`, `docs/web-ui.md`, `docs/tui.md`, `docs/filtering.md`, `docs/features.md`,
      `docs/commands.md` and `skills/tix`
