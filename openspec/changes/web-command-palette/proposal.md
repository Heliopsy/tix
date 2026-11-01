# A command palette in the browser

## Why

Getting anywhere in the web interface means knowing where it lives: a task is found by opening Tasks and
filtering, a settings section by opening Settings and scrolling, a view switch by opening the View menu.
The terminal interface has had a palette behind ctrl+k for a while. A reader who knows the name of the
thing they want should be able to type it.

## What Changes

- ctrl+k, and command+k on a Mac, open a command palette on every signed-in screen, in every key scheme.
  A Search button under the brand in the sidebar, and in the bar on a phone, opens it too. The "?"
  overlay and the scheme table on Settings list it.
- The palette is a native modal dialog with a search box over one list grouped into Tasks, Projects,
  Screens and Actions. Screens are every sidebar destination the reader's sidebar shows and every section
  of Settings. Actions are New task, New task in a named project, New project, Open the filter, Show
  tasks as a list, a board or in lanes, Switch the colour scheme, Keyboard shortcuts and Sign out. Each
  action drives a control the interface already has.
- Screens and actions are embedded in the page, already translated, and are matched as the reader types
  with a fuzzy match in the browser. Projects are read once when the palette opens. Tasks are asked for
  only once something is typed, 150ms after the last keystroke, at most ten at a time.
- Arrow keys move the selection, Enter chooses, Escape closes, a click chooses, focus stays inside the
  palette and returns to where it was. What was chosen recently comes first, and recently viewed tasks
  are read from `tix.recentTasks()` when the page offers it.
- A task chosen in the palette honours Task opens in: the dialog when it is Dialog, the page otherwise.
- Two JSON routes behind the session, `GET /palette/tasks?q=` and `GET /palette/projects`. They add no
  capability: tasks are `ListTasks` with the query read by the filter language, plus `GetTask` for a
  typed ref, and projects are `ListProjects`. The API already answers the same questions with
  `GET /api/v1/tasks?q=&limit=10`, `GET /api/v1/tasks/{ref}` and `GET /api/v1/projects`, and the CLI with
  `tix task ls --filter`.
- The shortcut layer now reads command as a modifier, so command plus a letter on a Mac no longer runs
  the bare letter's shortcut.

## Impact

- `internal/web`: `palette.go` (new), `routes.go`, `render.go`, `shortcuts.go`, `i18n.go`, templates
  `layout.html` and `partials.html`, assets `palette.js` (new), `decide.js`, `shortcuts.js`,
  `taskopen.js`, `app.css`, `icons.svg`.
- `internal/i18n`: Bulgarian entries for every new text.
- `docs/web-ui.md`, `docs/features.md`.
- No service, store, API, CLI or migration change. The TUI keeps its own palette, which already lists its
  actions; finding a task there is the filter bar.
