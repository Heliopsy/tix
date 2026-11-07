# Project task import and export

## Why

Getting one project's tasks into a spreadsheet, a report or another tracker means `tix task ls -o json`
and a script, and the listing's CSV has a fixed set of columns with the title first. The tenant snapshot
is the other way out, and it is tix's own NDJSON for backup, not a file anybody else reads. Coming the
other way, a CSV out of Jira, GitHub, Trello or Linear has nowhere to go short of writing a sync mapping.

## What Changes

- **Export a project's tasks** (`ExportTasks`) as CSV, JSON, NDJSON or a Markdown table. The selection
  takes the task list's query language and switches for finished, sub and trashed tasks; the caller picks
  the columns and their order (custom fields included), the date style (ISO, the day alone, or the
  reader's display format and zone) and the CSV separator. The file streams a page at a time. CSV cells a
  spreadsheet would run as a formula start with `'`.
- Surfaces: `POST /api/v1/projects/{ref}/tasks/export`; `tix export tasks`; an Export button and dialog in
  the project header with an up and down arrow per column; the `task_export` MCP tool, marked read-only.
- New authority `task.export` needs the `export` scope and is confinable, so a token pinned to a project
  exports its own project and no other.
- The MCP overlay gains a `read_only` key for an operation that only reads although it is a `POST`.
- **Import into a project** (`ImportTasks`) from CSV, JSON or NDJSON, including files Jira, GitHub, Trello
  and Linear write, or a pasted list of quick-add lines. Columns and values are mapped by a mapping file or
  by tix's suggestion; rows match tasks imported before by external id (stored in `external_refs`) or
  reference; a dry run previews; any row in error writes nothing. Surfaces: `POST
  /api/v1/projects/{ref}/tasks/import` (JSON or multipart), `tix import tasks`, five import steps in the
  browser with Back and Cancel on each, and `task_import` under MCP, destructive and so opt in.
- New authority `task.import` on the `import` scope, confinable. One `task.import` audit entry per run.

## Impact

- `core.TaskExportInput`, `TransferService.ExportTasks`, capability `task.export`, authz action
  `task.export`.
- New package `internal/taskio` for the writers.
- `internal/openapi`: a route that negotiates JSON or NDJSON as an alternative is described as records,
  not text.
- No migration.
