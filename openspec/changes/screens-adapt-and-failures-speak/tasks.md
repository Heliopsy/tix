# Tasks

## 1. The board strip

- [x] 1.1 `internal/web/assets/app.css`: `.board` as one flex row with `overflow-x: auto`, replacing
      the wrapping `auto-fit` grid that escaped the board's fixed height
- [x] 1.2 `internal/web/assets/app.css`: the phone rules carry `scroll-snap-type: x mandatory` and a
      column sized to the viewport, and no longer set `display: block`
- [x] 1.3 `internal/web/layout_test.go`: `TestBoardStaysOneStripOfColumnsAtPhoneWidth`, asserting the
      strip, the snap, and that the phone rules do not reintroduce the stack

## 2. The row that sheds

- [x] 2.1 `internal/web/templates/tasks.html`: a `.tm-*` cell per attribute, emitted even when empty,
      so a width rule can address one kind of attribute rather than a position in the row
- [x] 2.2 `internal/web/assets/app.css`: the tags and the updated stamp come off below 640px, the
      project chip below 480px

## 3. The failure that speaks

- [x] 3.1 `internal/web/templates/error.html`: the message in flow under its heading, not carrying
      the flash class that made it the corner toast
- [x] 3.2 `internal/web/assets/app.css`: `.failure` and its measure, and the ways out sharing the
      back-link's control definition rather than a second one of their own
