# Task text reads as Markdown, and a checklist can be ticked

## Why

Descriptions and comments were shown exactly as typed, in a `<pre>` or a pre-wrapped block, so a list,
a link or a code sample read as punctuation. Writers already type Markdown out of habit, and the most
common thing they type is a checklist of steps, which they then had to edit by hand in a textarea to
tick. Nothing outside the description could tell how far along those steps were.

## What Changes

- Descriptions and comments render in the browser as CommonMark with the GitHub tables, task lists,
  autolinks, strikethrough and fenced code. Raw HTML is shown as typed, only http, https and mailto
  links survive and carry `rel="noopener noreferrer nofollow"`, and images are never loaded: each
  becomes a link to its address. The markup carries no inline style or script.
- A task reference whose project the reader can see, and an `@handle` this tenant holds, become links.
- The editor stays a textarea, with a Preview toggle that renders on the server.
- Checklist items in a description render as checkboxes a reader with `task:write` ticks in place. A
  tick rewrites only that item's marker, carries the version it was drawn at, and a stale one is the
  usual conflict refusal.
- A new service operation, `CheckTaskItem`, with `POST /api/v1/tasks/{ref}/checklist`,
  `tix task check REF N` / `tix task uncheck REF N`, the browser's checkbox and the terminal's `[` key.
- A task read carries `checklist: {done, total}`; rows and cards show a `3/5` badge, cards a thin
  progress bar, `tix task show` states it and the terminal detail heads the body with it.
- Filter terms `has:checklist`, `checklist:open` and `checklist:done`, with `checklist` and
  `checklist_open` on the HTTP API.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-tasks`: Markdown rendering, Preview, ticking checklist items, the checklist badge.
- `http-api`: the checklist route, the `checklist` field and the two list parameters.
- `cli`: `task check`, `task uncheck`, the checklist line of `task show`.
- `task-filtering`: `has:checklist`, `checklist:open`, `checklist:done`.
- `tui`: the `[` key and the checklist count on the detail view.

## Impact

- New dependency `github.com/yuin/goldmark` (pure Go, no dependencies of its own, no cgo), behind
  `internal/markdown`.
- `internal/core` gains `Checklist`, `CheckItemInput`, two filter fields and the checklist markers.
- No migration: the count is computed from the body on read.
