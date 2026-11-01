# Design

## Layout

One grid with three areas: `props`, `main` and `meta`. At 1024px and up the grid is
`"main props" "main meta"` with a 20rem properties column; below it is a single column
`"props" "main" "meta"`. The properties come first in the document, so the phone order is also
the reading order for assistive technology, and the grid places them on the right on a wide screen.
In the dialog the same block is drawn, with the dialog's id prefix on every id.

## Editing in place without a script

Every editable property is a small form in its row that posts to `/tasks/REF` with the version,
the title and the one field it changes. The handler already treats an unnamed field as unchanged
(body, assignee, repeat, custom fields), so a row changes only itself. The due date joins that
list as `due_at`. With htmx, `data-inplace` swaps the page body as before; without it, the form
posts and redirects.

The field is drawn as text until pointed at or focused, and its Save appears then; Enter submits.
An enum custom field is a select with `data-apply`, which submits on change and is now rewired
after an in-place swap.

The title and the description edit behind `<details>` switches, which open and close without a
script. Their summary reads Rename or Edit closed and Cancel open; CSS hides the heading or the
rendered text while the editor is open.

## Live refresh

The refreshed regions stay `task-desc`, `task-pills`, `task-comments` and `task-history`.
`task-pills` is now the first block of the properties (status, priority, snooze and claim rows).
None of them holds a form carrying the version, and every menu their buttons open is drawn
outside them, as before.

## Trade-offs

- The Starred row is gone. The star control in the header carries the state in its pressed state
  and its accessible name (Star or Unstar), so the screen no longer says the same thing twice.
- History stays a disclosure in the main column; the main column is the wide one now.
- The project row is a link, not a control: moving a task between projects is Copy to.
