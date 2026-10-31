# Tasks

## 1. Preferences

- [ ] 1.1 `internal/web/render.go`, `session.go`, `routes.go`, `prefs.go`: `tix_task_open` and
      `tix_title_click`, their readers, toggles and entries in `Preferences`
- [ ] 1.2 `internal/web/templates/settings.html`: two switches in Listings

## 2. Task dialog

- [ ] 2.1 `internal/web/render.go`, `templates/task.html`: the `task-dialog` block for a request
      carrying `X-Tix-Dialog`, ids that collide prefixed, no live binding, the back link reads Close
- [ ] 2.2 `internal/web/templates/layout.html`: the dialog element on every screen but the task
      screen, in Dialog mode
- [ ] 2.3 `internal/web/assets/taskopen.js`, `decide.js`, `live.js`, `shortcuts.js`: open, close,
      history, focus, saves, refresh of the listing underneath, Enter on a selected row
- [ ] 2.4 `internal/web/assets/app.css`: the dialog, full screen under 600px

## 3. Title editing

- [ ] 3.1 `internal/web/taskdetail.go`: description and assignee left alone when not named
- [ ] 3.2 `internal/web/templates/partials.html`, `tasks.html`, `board.html`: the title partial
      with the edit marker and the open link
- [ ] 3.3 `internal/web/assets/taskopen.js`, `decide.js`, `app.css`: the field, save, cancel, errors,
      Enter and F2

## 4. Guards and docs

- [ ] 4.1 Go tests for the fragment, the preferences, the settings rows, the partial save and its
      conflict; JS tests for the decisions and the wiring; CSS guards
- [ ] 4.2 `internal/i18n`: Bulgarian entries
- [ ] 4.3 `docs/web-ui.md`
