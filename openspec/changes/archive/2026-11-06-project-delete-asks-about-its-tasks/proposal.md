# Deleting a project asks what happens to its tasks

## Why

Deleting a project removed the project row and let the database cascade through everything under it: every
task, soft deleted ones included, with their comments, attachments, tags, dependencies and stars. Nothing asked
first. The web danger zone was one button with no confirmation, `tix project rm` claimed exit 6 for "project still
holds tasks" but never refused, and the HTTP `DELETE` did the same. The tasks disappeared without an audit entry or
an event of their own, so nothing downstream learned they were gone.

A reader deleting a project usually wants one of two things: keep the work and fold it into another project, or
throw the whole thing away. The product should ask which, and do either one completely.

## What Changes

- `DeleteProject` takes an input naming what happens to the project's tasks: move them to another project, or
  delete them with it. A project holding tasks and given neither is refused with a precondition (exit 6), naming
  the count. An empty project deletes as before.
- Moving gives each task a new number in the target through the target's own counter, keeps its identifier and so
  its history, comments, attachments, tags, dependencies, stars, repeat rule and soft deletion, and maps its status
  onto the target's workflow: the same state when the target defines it, otherwise the target's first state of the
  same category, otherwise the target's initial state.
- Deleting with tasks is permanent, as project deletion always was, and every confirmation says so in those words.
- Every moved or deleted task gets its own audit entry and event, in the same transaction as the project's.
- A new read, `PlanProjectDeletion`, reports the task count and the status mapping before anything is written. It
  backs `tix project rm --dry-run`, `GET /projects/{ref}/deletion`, the web confirmation screen and the terminal
  confirmation.
- `tix project rm` gains `--move-tasks-to KEY` and `--delete-tasks`. `DELETE /projects/{ref}` accepts
  `move_tasks_to` and `delete_tasks` and returns what it did.
- The web danger zone's delete button opens a confirmation screen: the count, the two choices as radio buttons, a
  target project select with the mapping it implies, and the project key typed out for the delete path.
- The terminal removal form asks what happens to the tasks and its confirmation states the count and the mapping.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `task-management`: deleting a project decides the fate of its tasks.
- `cli`: `tix project rm` flags and refusal.
- `http-api`: the deletion parameters and the plan route.
- `web-ui`: the deletion confirmation screen.
- `tui`: the removal form and its confirmation.
- `capability`: the plan operation's bindings.

## Impact

- `internal/core`: `DeleteProjectInput`, `ProjectDeletion`, `StatusMove`, and the two `ProjectService` signatures.
- `internal/store`: `MoveTask` on both engines.
- `internal/service`: the deletion, the plan and the status mapping.
- `cmd`, `internal/httpapi`, `internal/client`, `internal/web`, `internal/tui`, `internal/capability`,
  `internal/i18n`, `docs/`.
- No migration.
