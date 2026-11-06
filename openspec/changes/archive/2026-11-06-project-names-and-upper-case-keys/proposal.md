## Why

The pickers a task is added from named a project by its key alone in some places and by name and
key in others, so a reader who knows their projects by name had to translate `infra` back to
Infrastructure, and one who knows them by key read a different order on every screen. Keys were also
drawn in the lower case they are stored in, while references read better and are conventionally
written upper case (INFRA-8), as tracker keys usually are.

## What Changes

- Every web control a project is chosen in names it in one style the reader picks on Settings:
  `Name (KEY)` (default), `KEY · Name`, `Name` or `KEY`. It is a per-browser cookie like the other
  display preferences. The option value stays the stored key.
- Project keys and task references are drawn upper case everywhere a reader sees them: the web
  interface, CLI tables and text, the terminal interface and calendar feed summaries.
- Stored keys, URLs, JSON, YAML, NDJSON and CSV values stay lower case, unchanged.
- Every place a key or reference is typed or followed takes it in any case: web addresses, filters,
  CLI arguments and flags, the TUI and the HTTP API. A few service paths that still matched a key
  exactly (copying a task, statistics, sync, choosing a task's project) now go through the same
  case-insensitive lookup.

## Impact

- Web: `internal/web/projectlabel.go`, template helpers `projLabel`, `key`, `ref`, `joinKeys`, the
  pickers, and the templates that draw a ref or key; palette, recent and forms scripts.
- CLI and TUI: `output.ShowKey` / `output.ShowRef` at render time.
- No migration, no API or wire change.
