# A task's lease can be set when it is created

## Why

A task's own lease length could only be set after the task existed, with `tix task edit --lease`, `PATCH` or the
task screen. An agent planning its work creates every step with `tix task add` and knows then how long each one
should take, so it had to follow every add with an edit or leave every step on the project default. A crashed
agent's quick step stayed held for as long as the default, and a long step expired mid-work.

## What Changes

- `CreateTaskInput` carries `lease`, held to the same maximum as an edit and refused above it.
- `tix task add --lease D`, `lease` on `POST /api/v1/tasks`, and a Lease field under More options on the
  browser's Add with details form.
- The terminal's new-task prompt reads quick-add syntax, which names no lease; docs/tui.md says where to set it.

## Impact

- `internal/core` (input), `internal/service` (create), `cmd`, `internal/web` (create form, Bulgarian strings).
- No storage change: the store already writes `tasks.lease` on insert.
