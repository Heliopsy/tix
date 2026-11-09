# Tasks

## Spec

- [x] Proposal, design and delta specs

## Service and store

- [x] `core.TaskFilter.PageNumber`, `core.TaskPage` total, capped flag and page
- [x] `CountTasks` and `SeekTask` on SQLite and PostgreSQL, capped count in the builder
- [x] `ListTasks` counts, seeks and clamps
- [x] Service tests: count with filters, scoped count, page N slice, clamp, page with cursor refused

## API, client, CLI, MCP

- [x] `page` query parameter, `total`, `total_capped`, `page` in the response; `just openapi`
- [x] Client reads the totals
- [x] `tix task ls --page`
- [x] HTTP test for the total field

## Web

- [x] Shared pager: N of M, numbered links with gaps, first/last, go-to GET form
- [x] Task screen seeks by page; every other paged listing counts by walking
- [x] View switch and lanes keep `page`; filter, Clear, narrow and Show all drop it
- [x] Bulgarian strings, CSS
- [x] Web tests: rendering, no-JS go-to, URL persistence, clamp, legacy cursor

## Terminal

- [x] Board status "page N/M", pgdown/pgup
- [x] TUI test for the indicator

## Docs

- [x] `docs/web-ui.md`, `docs/api.md`, `docs/tui.md`, the tix skill
