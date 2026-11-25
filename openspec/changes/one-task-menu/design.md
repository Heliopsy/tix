# Design

## One model, one template

`taskMenu` (internal/web/taskmenu.go) is built in the template from the screen being drawn and the
place: `lane`, `board`, `list` or `page`. It embeds the `laneCard` the lanes already decide with
(`newLaneCard`), so the Complete routes, the moves, the moves needing a comment and the scope flags
come from one function in every view. The list row's menu used to complete through `/complete` and
offer every workflow move whatever the reader's scopes; it now offers what the lane card offers.

The `task-menu` partial draws the items. `task-menu-button`, `task-menu-start` and `task-menu-end` are
the button and the popover frame, which the trash row reuses with its own two items.

## How a form swaps its answer

Two mechanisms already existed and both are kept. A lane card's forms carry explicit htmx attributes
(`taskMenu.HX`), because `assets/lanes.js` handles focus, the neighbour after a delete and refusals
inside the card for them. Everywhere else the form is plain and the region it sits in decides: a list
row and now a board card are `data-inplace`, and the task screen's menu carries
`data-inplace="main.wrap"`, so `assets/feel.js` arms the form. Without scripting every form is an
ordinary post returning to `next`.

## Per-view items

Only three things differ, each because it only makes sense in one place: Open the task is left off the
task screen; Select is on a list row only, just before Delete; the task screen adds Reclaim the lease
beside a live lease and Also delete subtasks under a task with subtasks, the only view that knows
either. Reclaim needs a reason field and lives in its own popover on the task screen, so it was not
spread to rows.

## Delete

Every Delete posts to `/tasks/REF/delete?next=...` with nothing else, so the handler's existing
`askBeforePermanentDelete` decides: trash on, soft delete and Undo; trash off, the confirmation. The
task screen's Delete permanently checkbox skipped that confirmation and is gone; a task in the trash is
purged from the trash row.

## Board drag

The drag handler read legal columns from the card's Move `<select>`. That select is gone, so it reads
the route buttons of the menu's Move section (`form[data-move-form]`) and posts that form with the
dropped column as `route`. The section offers only moves the reader may apply, so a column a reader
could not move to no longer lights up only to be refused.

## Keyboard

`z` on the task screen used to open the Snooze menu. The Snooze section's date field carries the
shortcut target now, and `pressTarget` opens the closed popover a target sits in before focusing it.
The palette's Snooze does the same. A delete shortcut was considered and left out: it would need a
binding in every key scheme, the shortcuts table and its docs, for something one menu press already
does.

## Trade-offs

The task screen now shows Copy link both in its menu and in the Share menu beside it, and field
controls (priority, assignee, due, tags) both in the properties and in the menu. That duplication is
the price of the menu reading the same everywhere.
