# Design

## What deletion did before

`Local.DeleteProject` authorised `project:write`, deleted the `projects` row and recorded one `project.delete` audit
entry and one `project.deleted` event. `tasks.project_id` is `ON DELETE CASCADE`, so the database removed every
task row of the project, live and soft deleted, and with them `comments`, `artifacts`, `task_tags`, `task_deps` and
`task_stars`. The tasks got no audit entry and no event. Project-scoped tags, which only an import creates, went
too. Nothing refused a project with tasks, although `tix project rm --help` said exit 6 meant exactly that.

## Why the project delete stays permanent

Making "delete the tasks too" a soft delete would mean soft deleting the project, since a task row cannot outlive
its project row under the cascade. That needs a `deleted_at` on projects, a filter on every project read, a rule
for a key reused while the old one sits in the bin, and a restore path on four surfaces. Archive already is the
reversible way to put a project away. So deletion stays what it was, permanent, and the confirmation says that in
plain words and points at Archive. What changes is that the tasks are accounted for: each one gets a `task.delete`
audit entry and a `task.deleted` event with `hard: true`, the same record `tix task rm --hard` writes.

## One method, an input, and a plan

`DeleteProject(ctx, ref, DeleteProjectInput) (*ProjectDeletion, error)`. The input carries `MoveTasksTo` and
`DeleteTasks`; both set is invalid, a target equal to the project is invalid, an archived target is a precondition
failure because moved tasks would vanish from every default view. The result reports the counts and the status
mapping that was applied, so the CLI and the API can print what happened.

`PlanProjectDeletion(ctx, ref, DeleteProjectInput)` runs the same resolution and counting without writing. It needs
`project:read`, and `task:write` on the target when one is named, because that is what the plan is for. It refuses
nothing the deletion would refuse for lack of a choice: a plan with no choice is how a reader learns the count.

The tasks are counted and walked including soft deleted ones. A soft deleted task in a moved project moves with the
rest and stays restorable in its new home; in a deleted project it goes for good, as it always did. One call is
bounded at 10000 tasks, the same bound a cascading task delete has.

## Moving

For each task in sequence order: reserve the target's next number with `NextSeq`, map the status, and write
`project_id`, `seq`, `status`, `updated_at` and `version + 1` through a new `MoveTask` store method on both engines.
The identifier does not change, so comments, artifacts, tags, dependencies (including those crossing projects),
stars, parents, repeat rules, leases and audit history all follow without being touched. The old reference, such as
`OLD-3`, stops resolving because its project is gone, which is the same as before.

Custom field values move as stored. The target's field definitions are not applied retroactively; the next edit of
the task validates against them like any edit.

A tag scoped to the deleted project would be cascaded off the task. Each one a moved task carries is re-created
under the target project with its name and colour and attached again.

Status mapping, decided in `internal/service`:

1. the same state key, when the target workflow defines it;
2. otherwise the first state of the target workflow, in its declared order, whose category equals the source
   state's category;
3. otherwise the target workflow's initial state.

Every moved task records a `task.move` audit entry and a `task.updated` event whose payload names the old and new
reference, project and status. No new event type: subscribers already handle `task.updated`, and a new type would
need an entry in the webhook catalogue for a change most consumers treat as an update.

## Authority

Unchanged for an empty project and for the delete path: `project:write`, then `project:write` on the project. The
move path additionally needs `task:write` on the target project, checked before anything is written.

## Surfaces

- CLI: `tix project rm KEY --move-tasks-to OTHER | --delete-tasks [--dry-run]`. Without either and with tasks it
  exits 6 and names both flags. `--dry-run` prints the plan.
- HTTP: `DELETE /projects/{ref}?move_tasks_to=OTHER` or `?delete_tasks=true`, `200` with the result.
  `GET /projects/{ref}/deletion` takes the same parameters and returns the plan.
- Web: the danger zone's Delete is a link to `GET /projects/{key}/delete`, a confirmation screen. An empty project
  gets one button. Otherwise the screen shows the counts, a radio for each choice, a target select, the mapping for
  the selected target, and a key field required on the delete path. Changing the target posts a preview intent that
  redirects back with `?move_to=`, so the CSRF token never lands in a URL; with scripting on the select submits it
  on change. The key check is a web confirmation, so it lives in the web handler.
- TUI: the removal form gains a "tasks" row, shown only for delete, offering "delete them" and "move to KEY" for
  each other live project. Agreeing to the form fetches the plan and the confirmation states it.
