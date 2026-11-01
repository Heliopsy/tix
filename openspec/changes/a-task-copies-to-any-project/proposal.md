# A task copies to any project

## Why

A reader who wants the same piece of work in another project, or a second copy of it beside
the first, retypes it: title, description, tags, fields, subtasks. The request was a copy
action that takes a task to any project the reader can write to, the same one included.

## What Changes

- New operation **copy a task**. The copy is a new task with a fresh reference in the target
  project. Title (editable), description, priority, tags, the custom fields the target defines,
  repeat rule, due date and assignee travel with it.
- Status is the target workflow's initial state, or with **keep status** the same-named state
  when the target workflow has one.
- Options: **with comments** (copied as new comments by the copier that name the original
  author and time), **with subtasks** (the live subtree, recursively, parent links kept),
  **with dependencies** (the copy waits for what the original waits for; edges inside a copied
  subtree point at the copies), **link** (the copy records the task it was copied from).
- Never copied: stars, claims and leases, history, artifacts, the recurrence successor link.
- One transaction: rows, a `task.copy` audit entry and a `task.created` event per new task (the
  event payload names `copied_from`), plus `comment.add` for every copied comment.
- New nullable column `tasks.copied_from_id` (migration 0020), shown as "Copied from" on the
  task screen's About card, in `tix task show` and in the API's task document.
- Surfaces: `tix task copy`, `POST /v1/tasks/{ref}/copy`, a "Copy to…" action and form in the
  web interface (page, or the task dialog when tasks open in a dialog), and the `Y` key in the
  terminal interface.

## Impact

- `core.CopyTaskInput`, `core.Task.CopiedFromID`, `Service.CopyTask`, capability `task.copy`.
- Migration 0020 on both engines; store task columns.
- No change for readers who never copy.
