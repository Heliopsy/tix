# Lane cards act in place

## Why

The lanes view drew a card that could do one thing: tick, and only when the workflow had exactly one
shortest route into done. A finished card could not be unticked, the status badge and the priority
were labels, an ambiguous workflow gave a dashed circle whose reason lived in a tooltip a touch
screen never shows, and a refused tick swapped the card for nothing, so it vanished. On a phone the
lanes were a fixed-height frame scrolling inside a page that also scrolled, three scroll axes for
one thumb. A reader called it a sorry implementation, and they were right.

## What Changes

- Ticking a finished card reopens it along the workflow's nearest route back to an open state. When
  several open states are equally near, the tick opens a menu offering them.
- A completion with more than one route into done offers the routes in the same menu instead of
  withholding the box.
- Where the workflow or the reader's scopes forbid a tick, the circle opens a panel that says why and
  links to the task, rather than being a dead link with a tooltip.
- The status badge is a menu of the moves this reader may make from the card's state, and the
  priority is a menu of the priorities, both the same popover controls the list rows use.
- Every lane action swaps only its own card, marks the card busy while the request is in flight,
  keeps focus on the control that was pressed, and shows a refusal inside the card.
- A live refresh waits while a request is in flight or a menu is open, and puts focus back where it
  was after it lands.
- A live refresh hands the markup it draws to htmx, so forms it drew keep swapping in place instead of
  posting as whole pages.
- Enter and Space on a focused button are no longer taken by the `open` shortcut.
- Below 640px the lanes snap sideways at their natural height and the page scrolls, instead of a
  fixed-height frame; lane controls are at least 40px on a coarse pointer.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-tasks`: the lane card's tick, status and priority controls, and the lanes layout on a phone.

## Impact

- `internal/web/tasklanes.go`, `internal/web/templates/tasks.html`, `internal/web/assets/app.css`,
  `internal/web/assets/lanes.js`, `internal/web/assets/decide.js`, `internal/web/assets/live.js`,
  `internal/web/assets/shortcuts.js`.
- `internal/i18n/bg_tasks.go` for the new strings.
- `docs/web-ui.md`.
- No service, store or API change: every control posts to the transition and priority routes that
  already exist.
