# A lane card has a menu that is always there

## Why

The View menu hides a lane card's fields, and the controls rode on the fields: hide Status and the
status menu goes with it, hide Priority and nothing on the card changes a priority, hide Star and
the star is gone. A reader who tidied the lanes down to titles was left with a tick and a link. They
asked for three dots on every card that keep full control of the task however the card is drawn.

## What Changes

- Every lane card carries a "more actions" button, three dots, drawn whatever the View settings
  hide. It opens a popover menu, the same component and keyboard behaviour as the badge and
  priority menus: Escape closes only the menu, focus returns to the dots, shortcuts stand down
  and a live refresh waits while it is open. Arrow keys move between its items.
- The menu opens the task (in the dialog when the reader chose Dialog), copies its link and its
  reference, completes or reopens it along the tick's routes or says why it cannot, moves it to any
  state the reader may reach from the lane, links to the task for moves that need a comment,
  changes priority, stars and unstars, renames the title in place, sets the assignee, adds and
  removes tags, and soft deletes it with the toast's Undo.
- Each item is a plain form or link that works without scripting. With htmx only the card is
  swapped, it dims while pending and a refusal is said inside it, like every other lane form.
- A reader sees only what they may do: moves need the transition scope, editing needs task write,
  deleting needs task delete.
- The edit, tag and restore routes return to the listing named by a submitted `next`, as the
  transition, priority and star routes already do, so a card can be changed in place and the
  toast's Undo of a delete brings the reader back to the listing rather than the task screen.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-tasks`: the lane card's more actions menu.

## Impact

- `internal/web/tasklanes.go`, `internal/web/taskdetail.go`, `internal/web/templates/tasks.html`,
  `internal/web/assets/app.css`, `internal/web/assets/icons.svg`, `internal/web/assets/lanes.js`,
  `internal/web/assets/taskopen.js`, `internal/web/assets/copy.js`, `internal/web/assets/decide.js`.
- `internal/i18n/bg_tasks.go` for the new strings.
- `docs/web-ui.md`.
- No service, store or API change. A due date is not offered: the web has no form that sets one.
