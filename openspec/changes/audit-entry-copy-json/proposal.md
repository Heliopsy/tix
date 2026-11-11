## Why

Pulling one audit entry out of the web UI meant opening Raw and pressing Copy all as JSON, and the TUI
had no way to get an event out at all short of retyping it. People paste these into bug reports and
chat often enough that it should be one press.

## What Changes

- Every audit row in the web UI (Activity, an actor's recent entries, a task's History) gets a Copy JSON
  button beside Raw that copies the row's entry as the API returns it, pretty-printed.
- The TUI activity view takes `^` to copy the selected event as JSON through OSC 52, like share.
- Docs mention the button, the key and the CLI's `-o json`.

## Impact

- `internal/web` (rawentry.go, partials.html), `internal/i18n`, `internal/tui`, docs.
