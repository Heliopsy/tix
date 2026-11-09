# Deleting a trashed task for good

## Why

A task in the trash can only leave it two ways: somebody restores it, or the trash rule expires it. Deleting
it again answers not_found, because a delete only looks at live tasks. Somebody who trashed a task holding a
leaked secret, or who simply wants it gone now, has to restore it and delete it again with `--hard`, or wait
out the rule, or turn the trash off.

## What Changes

- `PurgeTask` on the service permanently deletes one task already in the trash, with its trashed subtasks,
  deepest first, the same rows the trash rule removes, each with a `task.delete` audit entry and a
  `task.deleted` event (payload `hard`, `purged`), all in one transaction.
- A live task is refused as a failed precondition with reason `not_trashed`: it has to be deleted first.
- With the tenant's trash off it is refused with reason `trash_off`, like a restore: what the trash held stays
  untouched until it is back on.
- `DELETE /api/v1/tasks/{ref}/trash` (`task.purge`), `204` on success.
- `tix task purge REF...`.
- The browser's Trash gives each row a red Delete permanently button that opens a confirm dialog.

## Capabilities

### Modified Capabilities

- `task-management`: a trashed task can be purged.
- `http-api`: the purge route.
- `cli`: `tix task purge`.
- `web-tasks`: Delete permanently on a trash row.

## Impact

`internal/core` (method, error reason), `internal/service`, `internal/store` (the purge and trashed-children
queries take a zero cutoff as "any time"), `internal/httpapi`, `internal/wire`, `internal/client`,
`internal/capability`, `cmd/task.go`, `internal/web`, `api/openapi.yaml`, `docs/api.md`, generated CLI
docs, the tix skill.
