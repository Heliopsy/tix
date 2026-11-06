# Tasks

## 1. Server

- [x] 1.1 `internal/web/palette.go`: the embedded screens and actions, `/palette/tasks` and
      `/palette/projects`
- [x] 1.2 `internal/web/routes.go`, `render.go`: the routes and the embedded data on every screen
- [x] 1.3 `internal/web/shortcuts.go`: the palette action on ctrl+k and meta+k

## 2. Browser

- [x] 2.1 `templates/layout.html`, `partials.html`, `icons.svg`: the dialog, the opener, the data
- [x] 2.2 `assets/palette.js`: ranking, recent items, debounced search, keys, focus, actions
- [x] 2.3 `assets/decide.js`, `shortcuts.js`, `taskopen.js`: command as a modifier, the palette
      shortcut, the help overlay and the task dialog reachable from the palette
- [x] 2.4 `assets/app.css`: the opener, the dialog, touch targets, full screen on a phone

## 3. Words

- [x] 3.1 `internal/web/i18n.go`, `internal/i18n/bg_palette.go`: every new text in Bulgarian

## 4. Guards and docs

- [x] 4.1 Go tests for the routes, the limit, the tenant, deleted tasks, the markup and the language
- [x] 4.2 JS tests for ranking, recent items, no request until typing, the debounce and choosing
- [x] 4.3 `docs/web-ui.md`, `docs/features.md`
