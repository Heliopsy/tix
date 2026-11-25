# Move a task to another project

## Why

A task filed in the wrong project can only be copied, which leaves the original behind and starts a new
history, or carried along with every other task by deleting its project. Neither keeps the task. People file
into the wrong project all the time, and agents more so, so the task itself has to be able to move.

## What Changes

- `MoveTask` (`task.move`) moves one task into another project. It keeps its identifier and everything keyed
  by it: comments, artifacts, history, dependencies, stars, tags, assignee and the caller's own claim. It takes
  the target's next number.
- The old reference keeps resolving. A new table, `task_key_aliases`, keeps the project and number a task had
  before each move, and `GetTask` falls back to it, so links, mentions and scripts still reach the task. The
  web redirects the old address to the new one.
- The status is kept when the target workflow has a state of the same key, else the task starts in the
  target's initial state; the answer says which (`status_kept`).
- Custom fields the target defines and accepts keep their value; the rest are dropped and named in
  `dropped_fields`. The target's defaults and required fields apply as on a create.
- Tags scoped to the source project are recreated in the target, as moving a deleted project's tasks does.
- Needs edit rights on the task and create rights in the target. A task claimed by somebody else is refused
  with a conflict until the claim ends.
- One `task.move` audit entry with both references, and a `task.updated` event in each project so open
  boards on either side refresh.
- Surfaces: `tix task move REF... --to PROJECT`, `POST /api/v1/tasks/{ref}/move` (and so an MCP tool),
  Move to… on the web task screen, `move_project` in bulk actions and the web bulk bar, `d` on the terminal
  board and detail.
