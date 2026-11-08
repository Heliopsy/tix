# Design

## One purge path

The trash rule already deletes trashed tasks: `trashedSubtree` collects a task and its trashed descendants
deepest first, `PurgeTrashed` deletes each row only while it is still trashed, and the record is a
`task.delete` audit entry and `task.deleted` event with `hard: true`. The explicit purge calls the same three
pieces with no cutoff (a zero time, which both store engines read as "any time") so its effect is the
sweep's. Comments, links, attachments, stars and history go the way they go under the sweep: the foreign
keys on the task row decide, and the audit trail keeps its entries, which never cascade. A live child of a
trashed task is left alone and, as under the sweep, its parent link is cleared by the schema.

The record carries `purged: true` instead of the sweep's `cleanup` and `after_days`, so the trail tells a
person's purge from the rule's.

## Who may purge

The same `task:delete` permission and scope as a delete. Anyone allowed to delete a task can already delete
it permanently with `hard`, so a stricter rule here would only add a restore-then-hard-delete detour.

## What refuses

- A live task: `precondition_failed`, reason `not_trashed`. Purging a live task would be a hard delete with
  a different name; the caller is pointed at the delete.
- Trash off: `precondition_failed`, reason `trash_off`. Turning the trash off promises to keep what it held,
  out of reach, until it is turned on again, which is also why the rules pause then.
- There is no legal hold or minimum retention for tasks in tix. The trash rule is a maximum age, never a
  minimum, so it does not forbid an early purge. If a hold is added later, this is the one place to check it.

## Route

`DELETE /api/v1/tasks/{ref}/trash` sits beside `POST /api/v1/tasks/{ref}/restore`: the two ways out of the
trash, both keyed by the task. A DELETE is marked destructive for MCP without an overlay entry.

## Terminal

No board binding. The board reveals deleted cards through its filter and offers restore on them, but a
permanent delete there would need its own confirmation flow; purging is offered on the web Trash, the command
line and the API.
