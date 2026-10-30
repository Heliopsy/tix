# Tasks

## 1. The markup declares what the browser will do anyway

- [x] 1.1 `internal/web/templates/board.html`: the Move control's first route option carries
      `selected`, so the option the browser presents is the one the markup named

## 2. A control nobody touched does not read as edited

- [x] 2.1 `internal/web/assets/decide.js`: `fieldDirty`'s `SELECT` branch reads a control with no
      declared default by its `selectedIndex`, and keeps the option-by-option comparison where a
      default is declared

## 3. Guards

- [x] 3.1 `internal/web/boardlive_test.go`: the rendered board and its live fragment both give the
      Move select exactly one selected option, and it is the first
- [x] 3.2 `internal/web/jstest/decide.test.mjs`: an untouched select with no declared default is
      clean, the same select moved off the first option is dirty, and a declared default is still
      what a selection is read against
- [x] 3.3 `internal/web/jstest/live.test.mjs`: a freshly rendered board refreshes on its first
      event, and a board whose reader has picked a move still defers
