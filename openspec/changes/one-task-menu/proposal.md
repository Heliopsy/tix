# One task menu everywhere

## Why

The three dots on a task opened a different menu in each view. A lane card had the full set, Delete
included. A list row had no Delete, no Rename, no Priority and no Copy, but had Select. A board card's
three dots were a Move fold and nothing else. The task screen had its own menu with Copy to, Snooze as a
second menu, a Delete with two checkboxes and a Restore button for a task that was not deleted. Readers
could not tell what the dots would offer before opening them, and could not delete a task from the list
at all.

## What Changes

- One template draws the task menu for a lane card, a board card, a list row (the task list, My work,
  saved filters, a project's list) and the task screen and dialog. Same items, same order, same labels,
  same scope gating.
- The board card's Move fold becomes the three dots. Dragging a card reads its legal columns from the
  menu's Move section.
- A list row can now delete, rename, change the priority, copy the link and reference, and copy, move or
  template a task, like a lane card.
- The task screen's menu becomes the shared one: Snooze is a section of it rather than a second menu,
  Copy link is in it, and Delete uses the trash or the permanent-delete confirmation like everywhere
  else. The Delete permanently checkbox and the Restore button go; Also delete subtasks stays, under a
  task that has subtasks.
- A trash row's Restore and Delete permanently move behind the same three dots.

## Capabilities

### Modified Capabilities

- `web-ui`: one task menu, its order, its per-view extras, and its single delete path.

## Impact

`internal/web/taskmenu.go`, `internal/web/templates/partials.html`, `internal/web/templates/tasks.html`,
`internal/web/templates/task.html`, `internal/web/assets/live.js`, `lanes.js`, `taskopen.js`,
`shortcuts.js`, `palette.js`, the stylesheet parts for the board card and the task screen, the catalogs
in `internal/i18n`, and `docs/web-ui.md`.
