# Tasks

## 1. The preference

- [x] 1.1 `internal/web/taskview.go`: `TaskViewLanes`, a third switch position, and `taskViewOf` /
      `setTaskView` accepting any known view while the list still stores nothing

## 2. The lanes

- [x] 2.1 `internal/web/tasklanes.go`: `buildLanes`, one lane per project in screen order, empty lanes
      only for projects a `project:` term names, unknown-project tasks collected rather than dropped
- [x] 2.2 `internal/web/tasklanes.go`: `laneCategory`, the badge's category with the statistics
      derivation for a state that declares none
- [x] 2.3 `internal/web/tasklanes.go`: `completion`, exactly one shortest route to a terminal done
      state, withheld for a comment, a missing scope or an ambiguous choice, with the reason
- [x] 2.4 `internal/web/tasks.go`: lanes built from the same page and filter, exempt from the
      open-work default

## 3. The screen

- [x] 3.1 `internal/web/templates/tasks.html`: lanes, cards with the status badge, the tick form
      posting to the transition route, the locked link, the finished mark, the stray notice, and the
      Show completed toggle
- [x] 3.2 `internal/web/assets/app.css`: lane geometry shared with the board, phone snapping, and
      hiding finished cards under the root class
- [x] 3.3 `internal/web/assets/lanes.js`, `internal/web/templates/layout.html`: the stored toggle,
      applied from the head before the body paints
- [x] 3.4 `internal/web/templates/settings.html`: the help text names lanes

## 4. Guards

- [x] 4.1 `internal/web/tasklanes_test.go`, `internal/web/tasklanes_internal_test.go`: the switch offers and persists lanes; lanes group by
      project in order; visibility and named empty lanes; the badge; the tick box offered only for
      one route and completing the task; withheld for comment, scope and ambiguity; live fragment
- [x] 4.2 `internal/web/jstest/lanes.test.mjs`: the stored choice is applied, toggled, removed on
      the default, and survives storage that throws
- [x] 4.3 `internal/web/assets_test.go`: `lanes.js` is served with a validator

## 5. Documentation

- [x] 5.1 `docs/web-ui.md`: lanes, the tick box rules, Show completed and the storage reasoning, and
      the preferences table row
