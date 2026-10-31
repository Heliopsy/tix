# Tasks

## 1. In place

- [x] 1.1 `assets/feel.js`: arm forms in `[data-inplace]`, optimistic star, pill and priority,
      pending fade, rollback and toast on refusal or network failure, focus restored
- [x] 1.2 `templates/tasks.html`, `templates/task.html`: rows, lane cards (for the star) and the task screen declare what they swap;
      not in the dialog
- [x] 1.3 `templates/layout.html`: `#toast` always present; Undo button and form

## 2. Undo

- [x] 2.1 `taskdetail.go`, `tasks.go`, `render.go`: `flashundo` for moves, priorities and soft deletes;
      dropped from every address built from the request

## 3. Live refresh

- [x] 3.1 `assets/live.js`: keep the scroll of boxes in the region; focus through the shared `tix.focusMemo`
- [x] 3.2 `assets/decide.js`: `regionWaiting` also holds for `is-pending`; `focusMemo` maps a menu choice to
      its opener; `data-focus` on the star, row tick, status and priority controls

## 4. Navigation

- [x] 4.1 `assets/feel.js`: follow on press, view transitions without reduced motion, scroll kept on Back
- [x] 4.2 `assets/feel.js`: filter as you type; `templates/partials.html`: view switch boosted
- [x] 4.3 `assets/shortcuts.js`: overlay looked up when used; sidebar button opens it

## 5. Look

- [x] 5.1 `assets/app.css`: pending fade, star press, menu drop-in, toast leave, lighter shadows,
      view transition timing, all motion gated

## 6. Guards

- [x] 6.1 `inplace_test.go`, `undo_internal_test.go`, `feelcss_test.go`
- [x] 6.2 `jstest/feel.test.mjs`, `jstest/live.test.mjs`, `jstest/decide.test.mjs`,
      `jstest/shortcuts.test.mjs`
