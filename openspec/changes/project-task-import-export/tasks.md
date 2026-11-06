# Tasks

## 1. Spec

- [x] 1.1 Proposal, design and delta specs; `just spec` passes

## 2. Export

- [x] 2.1 `core.TaskExportInput`, columns, formats, date styles; `TransferService.ExportTasks`
- [x] 2.2 authz action `task.export` on the export scope, confinable to a pinned project
- [x] 2.3 `internal/taskio` writers: CSV with the formula guard, JSON, NDJSON, Markdown
- [x] 2.4 Service: query, inclusion switches, column cells, lookups memoised, keyset walk
- [x] 2.5 Registry entry `task.export`, HTTP route, client, `tix export tasks`
- [x] 2.6 Web Export button and dialog, column order arrows, refusal in the dialog, Bulgarian strings
- [x] 2.7 MCP: JSON and NDJSON alternatives described as records; overlay `read_only`
- [x] 2.8 Docs (import-export.md, web-ui.md, migrating.md, mcp.md), skill

## 3. Import

- [x] 3.1 Read CSV, JSON and NDJSON, detect Jira, GitHub, Trello, Linear and tix files
- [x] 3.2 Column and value mapping, auto-suggested; mapping file
- [x] 3.3 Dry run preview, upsert by external id, all-or-nothing commit, error report
- [x] 3.4 Surfaces: CLI, API (JSON or multipart), MCP (destructive, opt in), the import steps in the browser
- [x] 3.5 Pasted lists, notes that a few tasks need no import, Skip to preview, Back and Cancel on every step
- [x] 3.6 Docs (import-export.md, agents.md, migrating.md, web-ui.md, mcp.md), skill, Bulgarian strings
