# Tasks

## 1. Model and storage

- [ ] 1.1 `internal/core/view.go`: `GroupPref`, inputs, view names, `ViewService`
- [ ] 1.2 `internal/store/migrations/0016_group_prefs.sql` and the store methods on both engines
- [ ] 1.3 `internal/service/view.go`: the four methods, one transaction each with audit and event
- [ ] 1.4 `internal/authz`: `view.arrange`; `internal/capability/registry.go`: four entries

## 2. Surfaces

- [ ] 2.1 `internal/wire`, `internal/httpapi`, `internal/client`: the four routes
- [ ] 2.2 `cmd/view.go`: `tix view ls|set|order|reset`
- [ ] 2.3 `internal/web`: group menus on the board, the lanes, the project board and the grouped
      list; the list grouping preference; the Hidden control; routes and handlers
- [ ] 2.4 `internal/web/assets`: styles, `groups.js` focus return, i18n with Bulgarian
- [ ] 2.5 `internal/tui`: the project board honours the arrangement; palette commands

## 3. Documentation

- [ ] 3.1 `docs/api.md`, `docs/web-ui.md`, `docs/tui.md`, `docs/features.md`, `docs/commands.md`,
      README and `skills/tix`
