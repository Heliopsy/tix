# Tasks

## Service

- [ ] `PurgeTask` on `core.Service` and `core.ErrNotTrashed`
- [ ] Store: a zero cutoff on `TrashedChildren` and `PurgeTrashed` means any time, both engines
- [ ] `Local.PurgeTask` reusing the sweep's subtree, purge and record
- [ ] Service tests: purge with subtasks, audit and event, live task, trash off, permission

## Surfaces

- [ ] Route `DELETE /api/v1/tasks/{ref}/trash`, handler, client, HTTP test
- [ ] Registry entry `task.purge` with CLI, HTTP and Web bindings and a TUI exemption
- [ ] `tix task purge` and generated CLI docs
- [ ] Web: Delete permanently on each trash row, confirm dialog, Bulgarian strings
- [ ] `just openapi`, `docs/api.md`, docs, tix skill
