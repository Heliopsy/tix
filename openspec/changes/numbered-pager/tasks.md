# Tasks

## Spec

- [x] Proposal, design and delta specs

## Service and store

- [ ] `core.TaskFilter.PageNumber`, `core.TaskPage` total, capped flag and page
- [ ] `CountTasks` and `SeekTask` on SQLite and PostgreSQL, capped count in the builder
- [ ] `ListTasks` counts, seeks and clamps
- [ ] Service tests: count with filters, scoped count, page N slice, clamp, page with cursor refused

## API, client, CLI, MCP

- [ ] `page` query parameter, `total`, `total_capped`, `page` in the response; `just openapi`
- [ ] Client reads the totals
- [ ] `tix task ls --page`
- [ ] HTTP test for the total field

## Web

- [ ] Shared pager: N of M, numbered links with gaps, first/last, go-to GET form
- [ ] Task screen seeks by page; every other paged listing counts by walking
- [ ] View switch and lanes keep `page`; filter, Clear, narrow and Show all drop it
- [ ] Bulgarian strings, CSS
- [ ] Web tests: rendering, no-JS go-to, URL persistence, clamp, legacy cursor

## Terminal

- [ ] Board status "page N/M", pgdown/pgup
- [ ] TUI test for the indicator

## Docs

- [ ] `docs/web-ui.md`, `docs/api.md`, `docs/tui.md`, the tix skill
