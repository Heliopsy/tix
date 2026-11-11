# Design

## One mover, two callers

Project deletion already moves tasks: `NextSeq` in the target, `mapStatus` across workflows, scoped tags
re-created under the target, `store.MoveTask`, then a `task.move` audit entry and a `task.updated` event
carrying `ref`/`from_ref`, `project`/`from_project` and `status`/`from_status`. That loop becomes a
`taskMover` holding the source and target projects, both workflow definitions and the re-homed tags.
`DeleteProject` and `MoveTask` both drive it, so the two moves cannot drift apart.

## What moves

The named task and its live descendants that sit in the same project, in pre-order, so the root takes the
first new number. A descendant filed in another project stays there; its parent link already crossed
projects and still does. Soft deleted subtasks stay behind with their parent link, which a deleted task's
restore already tolerates. The walk is bounded by `maxSubtreeSize`, as every subtree walk is.

## The parent

Parent links may cross projects (project deletion keeps them), but a move is the reader saying the task
belongs elsewhere now. A copy into another project becomes a root task, so a moved task with a parent
does the same: it is detached before the move and its event names `from_parent`. Refusing would make
every client clear the parent first in a second write with its own version check, for no gain.

## Dependencies, stars, comments, history

Everything keyed by task identifier stays: dependencies (which may cross projects), stars, comments,
attachments, checklists, lease, custom field values and history. This matches project deletion.

## Authority and checks

The caller needs task update on every moved task in the source and on the target project, the same
target check project deletion makes. The target must exist in the tenant (the tenant scoped lookup
refuses anything else as not found), must not be the task's own project and must not be archived.
`version`, when given, is checked against the named task as `task.update` checks it and refuses a stale
one as a conflict. The whole move is one transaction.

## Old references

tix keeps aliases for renamed project keys only. A moved task's old reference answers not found; its
number is never issued again by the source's counter, so it cannot come to name another task. The
identifier does not change, and `GET /api/v1/tasks/{id}` keeps working. A task-level alias would need a
new table on both engines and is left for later.
