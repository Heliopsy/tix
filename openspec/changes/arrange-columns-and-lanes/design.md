# Design

## One table, keyed by view and group

`group_prefs (tenant_id, actor_id, view_name, group_key, hidden, collapsed, starred, sort_order,
updated_at)`, primary key on the first four. A row exists only while it says something: a change
that leaves a group plain deletes its row, so the table holds what readers chose and nothing else.

The view is stored as `board`, `lanes` or `project:<project id>`, and a project group as the
project's id. Both are read and written by key at every surface and translated in the service, so a
project renamed to a new key keeps its arrangement. A group that no longer exists (a state left the
workflow, a project was deleted) is ignored when drawing and still listed, so it can be reset.

Stars on project lanes are this table's `starred`, not a second project-star model: a project star
anywhere else would need its own meaning, and the request was about the lanes.

## Views name groupings, not drawings

The task screen draws one selection three ways. The list grouped by status and the board show the
same groups, so they share `board`; the list grouped by project and the lanes share `lanes`. A
project's own board is its own view, because its columns are one workflow's and the task screen's
board may be another.

## Order

Drawing order is: starred groups first, then groups the reader positioned by position, then the rest
in the view's own order. Move left and right swap a group with its drawn neighbour of the same star
state, and post the whole drawn order so the server writes positions for every drawn group. Groups
not drawn keep theirs. `OrderGroups` takes the order and writes positions 1..n for the named groups.

## Collapse keeps a drop target

A collapsed column is still a `.column` with its `data-state`, drawn as a narrow strip with no
cards, so a card dragged onto it moves there; live.js needs no change. The menu sits beside the
heading, never inside it, because live.js reads the column's name from the `h2`.

## In place

Every form keeps action and method and posts with a `next`, so it works without scripting. With
htmx it swaps the listing region (`#tasks-listing`, `#project-board`) out of the redirected page,
and the View menu's hidden section out of band. The menu is a popover inside the region, so an open
menu already holds a live refresh back (decide.js regionWaiting) and shortcuts stand down
(shortcuts.js). groups.js puts focus back on the dots of the same group after a swap, or on the
Hidden control when the group was hidden.

## Authorisation

A new action `view.arrange` needs `task:read`: the arrangement changes nothing anybody else sees. It
is not project confinable, since a view spans projects; a project-pinned token cannot arrange.

## Terminal interface

The terminal board is one project's board, so it reads and writes `project:KEY`: hidden columns are
not drawn, starred and positioned ones lead, a collapsed column is drawn as a narrow strip. The
palette offers hide, star, collapse, move left/right for the selected column and a reset.
