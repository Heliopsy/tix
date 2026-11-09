# A pager that says how many pages there are

## Why

The pager says "Page 1" with Previous and Next and nothing else. A reader cannot tell whether page 1 is
one of two or one of two hundred, and cannot get to page 12 without pressing Next eleven times. Paging
is keyset-based, which by itself knows neither the total nor how to land on page N.

## What Changes

- Every listing that renders the shared pager states "Page N of M · T tasks" (with the listing's own
  noun), draws numbered page links with gaps (`1 … 4 5 6 … 18`) plus First, Previous, Next and Last,
  and carries a small "Go to page" number field that is a plain GET form.
- The position is `page=N` in the address, so a page is linkable. The view switch and the lanes
  grouping keep it; the filter form, Clear, a project narrowing and Show all reset it to page 1. The
  live refresh keeps the reader on the page number they were on.
- The service counts a task listing with the same filter, scoped like the page itself, and can start a
  listing at page N. Both are keyset-safe: page N is translated to a cursor by a seek over the same
  ordering, and no statement emits `OFFSET`.
- The count is capped at 10,000 tasks. Past that the pager says "Page 3 of 400+" and the API marks the
  total as a floor.
- `GET /api/v1/tasks` answers with `total`, `total_capped` and, when asked by number, `page`; it
  accepts `page=N` beside the existing `cursor`, which stays the primary mechanism. MCP picks both up
  from the generated document.
- `tix task ls --page N` lists one page by number and says which page of how many it printed.
- The terminal board says "page N/M" in its status bar when a project holds more tasks than one read,
  and pages with `pgdown` and `pgup`.
- Listings other than tasks (activity, audit-backed sync runs, users, actors, tokens, deliveries,
  inbox, projects, tenants, account log) are counted and sought by walking their own keyset listing in
  500-row stretches, up to 1,000 rows, then "N+".
- A cursor link from before this change still opens the page it named.

## Capabilities

### Modified Capabilities

- `web-ui`: the shared position control counts pages and jumps.
- `web-tasks`: the task screen's page lives in the address and survives the view switch and refresh.
- `service-layer`: a task listing counts its selection and starts at a page number.
- `http-api`: the task listing's total and page parameter.
- `cli`: `tix task ls --page`.
- `tui`: the board's page indicator and page keys.

## Impact

`internal/core` (TaskFilter.PageNumber, TaskPage totals), `internal/store` (CountTasks, SeekTask on
both engines, a capped count in the builder), `internal/service/task.go`, `internal/httpapi`,
`internal/wire`, `internal/client`, `cmd/task.go`, `internal/tui`, `internal/web` (pager, every paged
handler, the pager partial, `140-pager.css`), `api/openapi.yaml`, Bulgarian strings, `docs/web-ui.md`,
`docs/api.md`, `docs/tui.md`, the tix skill.
