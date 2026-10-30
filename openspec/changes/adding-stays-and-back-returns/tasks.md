# Tasks

## 1. Back links

- [x] 1.1 `internal/web/render.go`, `templates/error.html`: the error screen's ways out are named
      for where they go, a failed page returns to its referrer or the task list, nothing links `/`
- [x] 1.2 `internal/web/taskdetail.go`, `templates/task.html`: the back link falls back to the
      referrer, survives saves through `taskPath`, and is labelled by destination; delete returns to it
- [x] 1.3 `internal/httpapi/middleware.go`: the not-found envelope replaces only the mux's own plain
      text answer, so the browser's HTML error screen reaches the reader

## 2. Adding stays on the list

- [x] 2.1 `internal/web/render.go`, `session.go`: absent means stay, an explicit on is honoured, the
      toggle writes `1` and `0`
- [x] 2.2 `internal/web/tasks.go`, `live.go`, `templates/tasks.html`, `assets/app.css`: the create
      redirect carries the toast's link and the row is marked; carried addresses drop the flash
- [x] 2.3 `internal/web/assets/live.js`: a refresh answer for a page already navigated away from is
      not drawn over the new one

## 3. Fields

- [x] 3.1 `internal/web/columns.go`, `templates/tasks.html`, `templates/partials.html`: Star and
      Repeats in View, the rule beside the glyph, cards respect the chosen fields
- [x] 3.2 `internal/web/taskdetail.go`, `templates/task.html`: starred, parent, next occurrence,
      last occurrence, started and finished on the task screen

## 4. Guards and docs

- [x] 4.1 Tests in `internal/web` for each of the above, each watched failing against a mutation
- [x] 4.2 `docs/web-ui.md`, `demo/scene/browser.mjs`
