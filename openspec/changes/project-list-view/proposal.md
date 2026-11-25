# A list on the project page, and filters that survive the view switch

## Why

A project's page is a board and nothing else. To read one project's work as rows, with a filter, a
sort or a search, a reader has to leave it for the task list and pick the project there. The board
on the project page also ignores every filter, because it has no filter bar.

On the task list the view switch is a form whose `next` is written when the page is drawn. The filter
bar narrows the listing in place with scripting on, so after typing a filter and pressing Board the
reader lands on the board with the filter gone. The view itself is a cookie, so a link to "this
filter, on the board" cannot be shared.

## What Changes

- `/projects/KEY` is the task screen scoped to the project: the same List, Board and Lanes, filter
  bar, search, sort, deadline window, saved filters, View panel (columns, grouping, arrangement),
  bulk bar, quick add, pager and live refresh, drawn by the same handler and template. The project's
  header (icon, name, workflow, Import, Export, Settings) stays on top.
- A `view` query parameter (`list`, `board`, `lanes`) picks the drawing on both screens and wins over
  the remembered choice. On the task list, following a switch link also remembers it in
  `tix_task_view`; the project page opens on its board when the address names no view.
- The view switch on both screens is a row of links, each carrying the current query with its own
  `view`. Filters a view does not use stay in the query, so switching back restores them. With
  scripting on, the filter bar's in-place refresh redraws the switch so its links carry the new query.
- `/projects/KEY/live` answers the scoped task screen's live fragment.
- The settings screen keeps its form for the remembered view.

## Capabilities

### Modified Capabilities

- `web-tasks`: the project page offers every view of the task screen, and the view travels in the
  address.

## Impact

`internal/web` (tasks.go, taskview.go, projects.go, groups.go, live.go, tasks.html, board.html,
partials.html, feel.js, palette.js, a new stylesheet part), `internal/web/i18n.go`, `docs/web-ui.md`,
the screenshot manifest. No service, HTTP API or CLI change.
