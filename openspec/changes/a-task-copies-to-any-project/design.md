# Design

## One operation, many rows

`CopyTask(ref, CopyTaskInput)` returns the root copy. Everything happens inside one
`l.write` transaction, so a copy of a subtree with comments either lands whole or not at all.
Each new task records `task.copy` in the audit log and a `task.created` event, because to a
subscriber a copy is a task that now exists; the payload carries `copied_from` (the source
reference) so a consumer that cares can tell. A copied comment records `comment.add` like any
other comment.

## What travels

| Attribute | Copy |
| --- | --- |
| title | the source's, or the caller's replacement |
| description, priority, tags, assignee, due date, repeat rule | as is |
| custom fields | only keys the target project defines and whose value its definition accepts; the target's defaults then apply, and a required field still missing refuses the copy |
| status | target workflow's initial state; `keep_status` keeps the same-named state when the target has it |
| parent | kept when the copy stays in the source's project, otherwise the copy is a root task |
| subtasks | `with_subtasks`: the live subtree, pre-order, each child under its parent's copy |
| comments | `with_comments`: new comments by the copier, prefixed with the original author's handle and time |
| dependencies | `with_dependencies`: the copy waits for what the source waits for; an edge whose far end is inside the copied subtree points at that copy; an edge the caller may not create is skipped |
| link | `link`: `copied_from_id` names the source |
| stars, claims, leases, history, artifacts, recurrence successor | never |

Comments keep their original author only as text. Writing them under the original author's
identity would put words in the audit trail under somebody who did not write them in that
place.

## The link is a column

The model has no typed relations. A link kept only in the audit log would vanish with
retention and be invisible to a reader without `audit:read`, so `tasks.copied_from_id` holds
it. It is not a foreign key, for the reason `recurrence_next_id` is not: a purged source
resolves to nothing, which every reader handles, and the copy keeps its own life. Snapshots
carry the field like every other task column.

## Authority

Read on the source, create on the target project, comment write on the target when comments
are copied, and task update on both ends of every copied dependency edge (the same rule
`AddDependency` applies). The capability scope is `task:write`.

## Surfaces

- CLI `tix task copy REF [--to PROJECT] [--title T] [--keep-status] [--with-comments]
  [--with-subtasks] [--with-deps] [--link] [--dry-run]`.
- HTTP `POST /v1/tasks/{ref}/copy` with the input as JSON, `201` with the root copy.
- Web: `GET /tasks/{ref}/copy` draws the form (project select defaulting to the current
  project, title prefilled, the options as checkboxes with link checked); `POST` copies. Inside
  the task dialog the form is drawn in the dialog. After copying the reader stays on the source
  with a toast linking the copy, or opens the copy when "open after add" is on.
- TUI: `Y` opens a form with the target project, title and the options.
