# A compact filter toolbar on the task screens

## Why

On `/tasks` and `/projects/KEY` the controls that decide what the list shows were spread over the page
header, a filter row, a Save this filter row, a lanes bar and two menus. On a laptop they stood between
the heading and the first row, and on a phone the header tools, the quick add and the filter button
wrapped into rows of their own. The owner measured it at about a third of the screen and found it hard
to read: the filter box, three selects with an (i) each, Apply, Clear, Save this filter, the view
switch, Group lanes by and View all sat at the same weight, and nothing on screen said which filter
was in force except the text in the box.

## What Changes

- One toolbar above the list, the same partial on the task list and the project page: a wide filter
  box, **Filters ▾**, **Sort ▾**, the view switch as a compact segmented control, the Select toggle
  where the screen has one, and **⋯** for the rest. With no filter it is one row on a desktop and at
  most two on a phone.
- **Filters ▾** is a native popover holding structured pickers for the project (not on a project
  page), assignee, tag, status, priority, deadline window and the open/all toggle. They are fields of
  the toolbar's one GET form; a picked status, priority, assignee or tag is folded into the filter
  expression by the server, which answers with a redirect to the canonical address.
- **Sort ▾** holds the sort field, a new direction (`order=desc`), the list grouping and, in Lanes,
  Group lanes by. **⋯** holds the columns, Export CSV, the page size, saved filters (save, apply,
  where to manage), the hidden groups and project visibility.
- Every active term is a removable chip in a slim row under the toolbar, with Clear all. A chip's ×
  is a plain link to the same address without that one term.
- The filter box suggests `project:`, `tag:`, `@person`, `status:`, `priority:` and `due:` values from
  the tenant, from data the page already loads. Arrows move, Enter or Tab takes a suggestion, Escape
  closes the list, and Enter with nothing chosen applies the filter.
- The filter language accepts `@handle` as `assignee:handle` (and `-@handle` as its exclusion) on
  every surface, since the parser is shared.

## Capabilities

### Modified Capabilities

- `web-tasks`: the toolbar, its popovers, the chips, typeahead, the sort direction and the fold.
- `task-filtering`: `@handle` is an assignee term.

## Impact

`internal/web/templates/tasks.html`, `internal/web/filterbar.go` (new), `internal/web/tasks.go`,
`internal/web/assets/filterbar.js` (new), `decide.js`, `feel.js`, a new stylesheet part, `icons.svg`,
`internal/query` (the `@` term and the term splitter), Bulgarian strings, `docs/web-ui.md`,
`docs/filtering.md`, `scripts/screenshots` (states and a toolbar height check).
