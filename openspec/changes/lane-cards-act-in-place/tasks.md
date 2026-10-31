# Tasks

## 1. Deciding the tick

- [x] 1.1 `internal/web/tasklanes.go`: `tickRoutes`, completion and reopening under one rule, with a
      menu for several candidates and a translatable reason when none is usable
- [x] 1.2 `internal/web/tasklanes.go`: the status menu's routes, filtered by comment and scope, and
      whether the reader may change the priority

## 2. The card

- [x] 2.1 `internal/web/templates/tasks.html`: tick box, tick menu and locked panel; status and
      priority popover menus; htmx attributes swapping the one card
- [x] 2.2 `internal/web/assets/app.css`: menu chips styled as the badges, busy card, error line,
      natural-height snapping lanes below 640px, 40px coarse-pointer targets
- [x] 2.3 `internal/web/assets/lanes.js`: focus back on the pressed control after a swap, refusal
      shown in the card
- [x] 2.4 `internal/web/assets/decide.js`, `internal/web/assets/live.js`: a refresh waits for an
      in-flight request or open popover, restores focus, and hands what it drew to htmx
- [x] 2.6 `internal/web/assets/shortcuts.js`: Enter and Space on a focused button are left to it
- [x] 2.5 `internal/i18n/bg_tasks.go`: Bulgarian for every new string

## 3. Guards

- [x] 3.1 `internal/web/tasklanes_internal_test.go`: the tick rule both ways, the status routes
- [x] 3.2 `internal/web/tasklanes_test.go`: untick, the menus, permissions, refusal, htmx partial
- [x] 3.3 `internal/web/jstest`: busy regions, focus memo and restore, lane swap handling
- [x] 3.4 CSS guards for the phone layout and touch targets

## 4. Documentation

- [x] 4.1 `docs/web-ui.md`: the lane card's controls and the phone layout
