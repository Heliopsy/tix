# Keep a page's memory to what it draws

## Why

tix 0.52.1 on a PostgreSQL deployment was killed for memory three times in fifty minutes at a 256 MiB container
limit, idling at about 82 MiB. The last kill followed a page load: `GET /inbox`, then `GET /inbox/badge`. The
tenant had just imported 963 tasks with long Markdown descriptions.

Every screen draws My work's overdue badge, and the live refresh fetches it again. The badge counted up to a
hundred overdue tasks by listing them, which read every description and parsed each one's Markdown to count its
checklist. On a seeded tenant of a thousand tasks with 5 to 50 KB descriptions that was about 50 MiB allocated
per screen, and `/inbox/badge` alone the same. Every task listing parsed its rows' descriptions the same way.
With the Go runtime's default pacing the heap may reach twice what is live before it collects, so two or three
such requests together were enough.

## What Changes

- The overdue badge lists its tasks without their descriptions.
- Counting a description's checklist parses only what decides which list items are checklist items, skips a
  text with no checkbox at all, and keeps the counts of long texts in a bounded cache by text hash.
- `tix serve` sets the Go runtime's soft memory limit to 85% of the container's cgroup memory limit at start,
  unless `GOMEMLIMIT` is set, and logs what it chose.

## Capabilities

### Modified Capabilities

- `server`: the soft memory limit follows the container's.
- `web-ui`: the sidebar badges do not read task descriptions.

## Impact

`internal/markdown/checklist.go`, `internal/web/sidebar.go`, `internal/core/filter.go`, both stores'
`ListTasks`, `internal/memlimit`, `cmd/serve.go`, `docs/deployment.md`.
