# A filter term for the trash

## Why

`is:deleted` widens a listing to include soft-deleted tasks alongside the live ones. There is no
way to ask for the deleted ones alone, which is the question somebody actually has when they go
looking for something they deleted: the docs had to suggest piping `-o ndjson` through `jq`. The
browser had no way to reach deleted tasks at all, so the Restore button on the task screen was
only reachable by somebody who already knew the task's ref.

## What Changes

- The filter language gains `is:trashed`, which selects only soft-deleted tasks, and
  `-is:trashed`, which selects only live ones. When either is present it decides, so
  `is:deleted -is:trashed` lists live tasks only.
- The task filter contract gains a tri-state `Trashed` field, answered by both stores in SQL and by
  the in-memory matcher the terminal uses. Keyset pagination is unchanged.
- CLI: `tix task ls --trashed`, beside `--filter is:trashed`. HTTP API: `trashed=true|false` on
  `GET /api/v1/tasks`. Terminal: the board filter takes the term like any other.
- Web: a Trash entry in the sidebar opens the task list filtered by `is:trashed`. That listing is
  titled Trash, says the rows are deleted, marks each row deleted and offers Restore on it, which
  posts to the existing restore route and returns to the trash.
- Docs, the agent skill and the Bulgarian catalog follow.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `task-filtering`: a term selecting only deleted tasks.
- `cli`: `tix task ls --trashed`.
- `http-api`: the `trashed` listing parameter.
- `tui`: the board filter answers the term.
- `web-tasks`: the Trash screen and Restore on a deleted row.

## Impact

`internal/core/filter.go`, `internal/query`, `internal/store/sqlite`, `internal/store/postgres`,
`internal/httpapi`, `internal/client`, `cmd/task.go`, `internal/web` (sidebar, task list, icons,
catalog), `internal/i18n`, `docs/filtering.md`, `docs/web-ui.md`, `docs/tui.md`,
`skills/tix`.
