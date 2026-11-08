# Tasks

## Service

- [x] `PurgeTask` on `core.Service` and `core.ErrNotTrashed`
- [x] Store: a zero cutoff on `TrashedChildren` and `PurgeTrashed` means any time, both engines
- [x] `Local.PurgeTask` reusing the sweep's subtree, purge and record
- [x] Service tests: purge with subtasks, audit and event, live task, trash off, permission

## Surfaces

- [x] Route `DELETE /api/v1/tasks/{ref}/trash`, handler, client, HTTP test
- [x] Registry entry `task.purge` with CLI, HTTP and Web bindings and a TUI exemption
- [x] `tix task purge` and generated CLI docs
- [x] Web: Delete permanently on each trash row, confirm dialog, Bulgarian strings
- [x] `just openapi`, `docs/api.md`, docs, tix skill
