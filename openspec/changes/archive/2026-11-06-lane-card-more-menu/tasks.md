# Tasks

## 1. Server

- [x] 1.1 `internal/web/tasklanes.go`: what the reader may write and delete, and the moves that need a comment
- [x] 1.2 `internal/web/taskdetail.go`: edit, tag, untag and restore return to a posted `next`

## 2. The card

- [x] 2.1 `internal/web/templates/tasks.html`: the dots and the menu, every item a form or link swapping the card
- [x] 2.2 `internal/web/assets/icons.svg`, `internal/web/assets/app.css`: the dots icon, the menu's sections, 40px targets on a coarse pointer
- [x] 2.3 `internal/web/assets/lanes.js`: arrow keys in a lane menu, links close it, focus after a delete
- [x] 2.4 `internal/web/assets/taskopen.js`: rename from the menu
- [x] 2.5 `internal/web/assets/copy.js`, `internal/web/assets/decide.js`: copy a given text or an absolute link
- [x] 2.6 `internal/i18n/bg_tasks.go`: Bulgarian for every new string

## 3. Guards

- [x] 3.1 `internal/web/tasklanes_test.go`: the menu with every field hidden, per scope, each action in place
- [x] 3.2 `internal/web/jstest`: menu arrow keys, absolute link
- [x] 3.3 CSS guard for the coarse pointer target

## 4. Documentation

- [x] 4.1 `docs/web-ui.md`: the lane card's menu
