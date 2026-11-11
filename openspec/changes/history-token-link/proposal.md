## Why

The activity feed links each row's channel to the feed narrowed to that channel and token, but a task's History
showed the same "via token ci-agent" words with no link, so following what one token did meant retyping its name.
Clients outside this repo also had no written statement of which fields name the token.

## What Changes

- A task's History links each row's channel to the activity feed narrowed to that channel and token.
- docs/api.md names `via.kind`, `via.token_name` and `via.token_id` as the fields a client reads from audit entries
  and events, and says a comment's channel lives on its `comment.add` entry.

## Impact

- Web: `internal/web/via.go`, `internal/web/templates/task.html`.
- Docs: `docs/api.md`, `docs/web-ui.md`.
- No API, CLI, TUI or schema change: those already carry and show the token name.
