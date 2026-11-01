# Snooze a task until later

## Why

A task that cannot move until next week still sits in every listing, on every board and in the
agents' claim queue, so people either delete it, park it in a fake status, or learn to read past
it. Snoozing hides it until a moment the reader picks and brings it back by itself.

## What Changes

- A task carries `snoozed_until` and `snoozed_by`. Snoozing is per task, not per reader: everyone
  stops seeing it, and the audit entry names who snoozed it.
- Every default task listing (CLI, HTTP, web list, lanes and board, TUI) leaves out a task whose
  `snoozed_until` is still in the future. The comparison is made at read time, so a task comes back
  the moment its time passes, whether or not anything else runs.
- The filter language gains `is:snoozed` (only snoozed tasks), `-is:snoozed` and `is:awake` (only
  tasks not snoozed, the default made explicit), `snoozed:true|false` and `snoozed:any` (both,
  widening the listing the way `is:deleted` does). The CLI has `--snoozed` and `--include-snoozed`,
  the HTTP API `snoozed=true|false|any`.
- `SnoozeTask`, `UnsnoozeTask` and `WakeSnoozed` join the service. Snoozing writes `task.snooze`
  and `task.snoozed`, unsnoozing `task.unsnooze` and `task.unsnoozed`.
- The server's sweeper loop, the one that already materialises lease expiry, also clears snoozes
  whose time has passed and emits `task.woke` for each. Without a running server nothing is
  emitted, but the task is already back in the listings.
- A snoozed task cannot be claimed: `claim next` skips it and `claim REF` refuses it until it
  wakes or is unsnoozed.
- CLI `tix task snooze REF UNTIL`, `tix task unsnooze REF` and `tix task wake`; HTTP
  `PUT`/`DELETE /api/v1/tasks/{ref}/snooze` and `POST /api/v1/snoozes/wake`; the TUI's `z` key;
  the web task screen, the lane card's more actions menu and the list row, with presets (later
  today, tomorrow morning, next week, a picked date and time) and Unsnooze. A snoozed task drawn
  anywhere carries a badge with its wake time.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `task-management`: snoozing and unsnoozing a task.
- `task-filtering`: the snooze terms and the default that hides snoozed tasks.
- `claim-lease`: a snoozed task is not claimable.
- `event-stream`: `task.snoozed`, `task.unsnoozed`, `task.woke`.
- `data-model`: the two columns, migration 0023.
- `http-api`, `cli`, `tui`, `web-tasks`, `capability`: the bindings.

## Impact

- Migration `0023_task_snooze.sql`; both store engines; `internal/service/snooze.go`;
  `internal/query` for the terms and the time parser shared by CLI and TUI; `internal/server` for
  the sweep worker.
- Docs: `docs/filtering.md`, `docs/features.md`, `docs/api.md`, `docs/web-ui.md`, `docs/tui.md`,
  `docs/agents.md` where claiming is described, `skills/tix/SKILL.md`.
