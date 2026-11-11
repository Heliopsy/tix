# Design

## Reusing the project deletion path

Deleting a project with `move_tasks_to` already renumbers tasks into another project through
`store.Tx.MoveTask` (project, number, status, version) and rehomes project-scoped tags. A single move uses
the same store call and the same tag rehoming, factored into `rehomeTags`. The status rule differs on
purpose: project deletion maps by category because the source workflow goes away, while a single move keeps
the same key or falls back to the target's initial state, which is the rule `task copy --keep-status` uses
and is easy to predict.

## Old references

`project_key_aliases` already lets a reference written under a project's old key resolve after a rename.
A moved task needs the same for its old number. `task_key_aliases (tenant_id, project_id, seq, task_id)`
records the project and number the task left. `GetTask`, after missing on the current key and number, now
resolves the project by key or key alias and looks for a task there first, then for an alias row. A project
never reissues a number (`seq_counter` only rises), so an alias cannot shadow a later task. Rows go with the
task or the project through `ON DELETE CASCADE`. Moving a task back into a project it left gives it a new
number there; the old alias still points at it.

Everything that takes a reference goes through `GetTask`, so the CLI, API, MCP, TUI palette and web all
resolve the old reference. The web task screen already redirects a reference that is not the task's current
one (`redirectOldRef`), so the old address answers 301 to the new one.

## Custom fields

Values are kept only where the target defines the field and the value coerces to its type; the others are
listed in `dropped_fields`, sorted, and recorded in the audit payload. The target's defaults and required
fields then apply as on a create, so a target that requires a field the task does not carry refuses the move
with the same invalid error a create gives. Nothing is silently invented.

## Claims

A live claim held by somebody else refuses the move with a conflict: the holder is working on it, and
moving it under them would change its reference mid-work. The caller's own claim, and an expired one, stay on
the task unchanged, since the identifier and lease token do not change.

## Subtasks and parents

Only the named task moves. Parents may already be in another project, so a moved task keeps its parent, and
its subtasks stay put and keep pointing at it.

## Authorization

`task.update` on the task in its project and `task.create` in the target, so a token pinned to one project
cannot move tasks out of it, and a viewer cannot move anything. The registry declares `task:write`.

## Events

One `task.move` audit entry and a `task.updated` event in the target project, plus a second `task.updated`
event carrying the same payload in the source project, so live boards subscribed to either project refresh.
The payload carries `ref`, `from_ref`, `project`, `from_project`, `status`, `from_status` and `status_kept`.

## Terminal key

`d`, for dispatch. `m` and `M` are the comment keys, `t` and `T` are taken, and `d` was free on every
view. It only opens a project picker, so a reflexive press does nothing.

## Bulk

`move` in bulk actions already means a status move, so the project move is `move_project`, with its new
reference in the outcome's `moved_ref`.
