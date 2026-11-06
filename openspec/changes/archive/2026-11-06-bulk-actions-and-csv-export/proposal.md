# Bulk actions and CSV export

## Why

A reader triaging a list changes one row at a time: ten tasks to `done` is ten status menus,
retagging a sprint is a tag form per task. The command line already takes many references for
most verbs, the browser takes none, and the API needs one request per task. Getting a filtered
list out into a spreadsheet means `-o json` and a script.

## What Changes

- New operation **bulk task action** (`BulkTasks`): one request names an action and up to 500
  task references; the service applies the action to each task on its own and answers with an
  outcome per task. Actions: move to a status, set the priority, add or remove a tag, set or
  clear the assignee, star, unstar, copy to a project, delete (soft) and restore.
- Every task keeps its own transaction, audit entry and event: the bulk operation calls the
  per-task service methods, so a refusal for one task never rolls back another.
- Surfaces: `POST /v1/tasks/bulk`; `tix task bulk ACTION REF...`; selection checkboxes and a
  sticky action bar on the web task list with one Undo for a bulk delete. The terminal
  interface acts on one card at a time and is exempted.
- CLI gaps closed: `tix task edit REF...` and `tix task copy REF...` take many references,
  `tix tag rm REF... TAG` takes many references, and `tix tag add` takes a comma separated
  list or `-` in the reference position.
- **CSV export**: `tix task ls -o csv`, `GET /v1/tasks?format=csv` (or `Accept: text/csv`) and
  Export CSV in the web task list's View menu. Cells are escaped by RFC 4180 and a cell starting
  with `=`, `+`, `-`, `@`, a tab or a carriage return is prefixed with `'` so a spreadsheet does
  not evaluate it.

## Impact

- `core.BulkTaskInput`, `core.BulkResult`, `Service.BulkTasks`, capability `task.bulk`.
- `output.FormatCSV`, accepted by `tix task ls` only.
- No migration, no change to existing per-task routes.
