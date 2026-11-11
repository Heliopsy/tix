# Move a task to another project

## Why

Clients want "Move to project". Today the only way a task changes project is deleting its whole project
with `move_tasks_to`. `task.update` has no project field, the bulk `move` is a status move and
`task.copy` makes a new task, so a task filed in the wrong project has to be copied and the original
deleted, losing its identifier, history, comments, stars and links.

## What Changes

- New service operation `MoveTask` (`task.move`): moves one task and its live subtree within its
  project into another project of the same tenant, reusing what project deletion does when it moves
  tasks: the target's next numbers, the status mapped into the target workflow, project-scoped tags
  re-created in the target, a `task.move` audit entry and a `task.updated` event per task.
- A task that has a parent is detached from it and becomes a root task in the target, as a copy into
  another project does. Its subtasks keep their parent links.
- `POST /api/v1/tasks/{ref}/move` with `{project_ref, version}`, answering 200 with the moved task under
  its new reference. MCP derives its tool from the route.
- CLI `tix task move REF... --to PROJECT [--version N] [--dry-run]`.
- Web: "Move to…" beside "Copy to…" in the task screen's menu opens a small form with the target project.
- The terminal board does not bind it; the exemption names the other surfaces.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `task-management`: moving a task to another project.
- `http-api`: the move endpoint.
- `cli`: `tix task move`.
- `capability`: the `task.move` registry entry.
- `web-tasks`: Move to another project in the browser.
- `event-stream`: the event a moved task emits.

## Impact

- `internal/core`, `internal/service` (`move.go`, shared helpers out of `projectdelete.go`),
  `internal/wire`, `internal/httpapi`, `internal/client`, `internal/capability`, `api/openapi.yaml`,
  `cmd/task.go`, `internal/web`, `internal/i18n`, docs and the skill.
- No migration. Old references do not resolve after a move: tix keeps aliases for renamed project keys,
  not for single tasks. The task identifier is unchanged and keeps resolving.
