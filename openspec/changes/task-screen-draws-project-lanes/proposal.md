# The task screen draws project lanes

## Why

The task screen draws one selection two ways: rows, or a board of workflow columns. Neither answers
the question somebody running several projects at once asks most often, which is "what is on each
of my lists". The list mixes every project into one ordering, and the board groups by state, so a
project's work is scattered across every column and two projects whose workflows differ cannot share
a board at all.

The arrangement people already know for this is one column per list with a tick box on every item,
the way Google Tasks draws it. Grouping by project instead of by state also sidesteps the board's
hardest problem: a lane never needs two workflows to agree, because it never draws a workflow.

## What Changes

- The view switch gains a third position, **Lanes**, stored in the same `tix_task_view` cookie as
  `lanes`, so a refresh draws lanes from the server with nothing to fix up in the browser.
- **One lane per project**, in the order the screen already lists projects, each holding that
  project's tasks from the current page in the listing's own order. The filter, deadline window,
  sort, page, page size and project visibility choice are resolved once, exactly as for the list and
  the board, and one pager walks all three views.
- A lane is drawn for every project with a task on the page. A project the filter names explicitly
  with `project:` gets a lane even when it is empty, because the reader asked for it by name. Other
  empty projects get none, so a tenant with hundreds of projects does not draw hundreds of empty
  columns.
- Since the lane is the project, **a card shows its status as a badge**, using the state's label and
  the category colour the rest of the interface already uses.
- **A tick box completes a task** when its own workflow offers exactly one shortest route from its
  current state to a terminal done-category state, and every step of that route is one this reader
  may take without a comment. Otherwise the box is replaced by a link to the task, which says why.
  Ticking posts to the existing transition route, as a plain form with CSRF, so it works without
  JavaScript. A finished task renders ticked. Unticking is not offered.
- Like the board, lanes are exempt from the list's open-work default: finished tasks are part of the
  drawing, ticked.
- **Show completed**: a per-browser toggle, kept in `localStorage` as one boolean, that hides
  finished cards in every lane. It is applied before the page paints, and with JavaScript off it is
  absent and finished cards are shown.
- Lanes refresh live through the existing task screen fragment, like the list and the board.

## Impact

- `internal/web`: `taskview.go`, a new `tasklanes.go`, `tasks.go`, `templates/tasks.html`,
  `templates/layout.html`, `templates/settings.html`, `assets/app.css`, a new `assets/lanes.js`.
- `internal/web/jstest`: a suite for `lanes.js`.
- `docs/web-ui.md`.
- No service method, route or capability is added. The tick box uses `POST /tasks/{ref}/transition`.
