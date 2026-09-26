# The four remaining pieces of daily work the terminal could not reach

## Why

The capability ratchet stood at forty-four terminal gaps. Most of what remains is administration, bulk data
movement, or a state machine no single-column form gathers. Four were not: they were ordinary work, inside
the scopes an SSH visitor already holds, and each was blocked on something the interface has since grown.

- `audit.list` said "there is no history view on a task or a project". The activity view was proposed as the
  place for it, and it is the wrong place. Activity is the live tail of a subscription, offered on
  `event:subscribe`, capped at two hundred events, and empty until something happens while you watch.
  History is the stored log, offered on `audit:read`, and it answers what happened before this session
  opened. A reader holding `audit:read` and not `event:subscribe` would have been offered no history at all,
  and a reader holding only `event:subscribe` would have been promised a record the service refuses.
- `artifact.put` said it needed structured input the interface did not have. It has a form now, and
  `core.ArtifactInput` requires exactly one thing: a kind, drawn from `core.ArtifactKinds`.
- `task.restore` needed somewhere to see deleted tasks. That somewhere already existed and nobody had
  noticed: the board's filter bar takes `is:deleted`, through the same parser `tix task ls --filter` uses.
  What was missing was a marker on a deleted card and a key to bring it back.
- `actor.list` was recorded as needing a directory view. A directory nobody navigates to earns no screen.
  What it earns is the assignee prompt, which took an actor identifier: a question nobody at a terminal can
  answer, because a reader knows a colleague by handle and has never seen the identifier the store holds.

## What Changes

- **A history view.** `H` reads one page of the durable log for whatever the reader is looking at: the
  selected task, the open project, or the tenant. Actor identifiers resolve to handles the way the detail
  view resolves them, and the view says when the log runs past its page rather than drawing a bounded
  listing as the whole of one. It is a screen of its own, not a section of the activity view, because the
  two answer different questions from different sources under different scopes.
- **Recording an artifact.** `O` takes a name at the prompt and a kind at the form, which is every field the
  service requires. The payload, the content type and the inline blob are not gathered, and that is recorded
  as a limitation on the binding rather than left for a reader to discover.
- **Restoring a deleted task.** A deleted card carries `†` and the legend explains it. `u` restores the
  selected card, and refuses a card that was never deleted while naming the filter that reveals the ones
  that were. The footer on a deleted card offers the restore and none of the editing keys the service would
  refuse on it.
- **An assignee picker.** `A` lists the tenant's actors and offers them by handle, with "unassigned" first
  so clearing an assignment is a choice on the same list. The picker opens on whoever holds the task now,
  and an assignee the directory no longer holds falls back to nobody rather than to the first name on it.

## Impact

- `internal/tui`: `history.go`; the assignee, artifact and restore wiring; `Restore`, `Artifact` and
  `History` in the key map, in every view's collision set, and in the gated action lists the footer, the
  overlay and the keystroke all read.
- `internal/capability/registry.go`: four terminal gaps become bindings, one of them carrying a recorded
  shortfall, and the gap count falls from 44 to 40.
- `internal/tui/prompt.go`: the assignee prompt is gone. It asked for an actor identifier, which the picker
  replaces; the operation it reached, `task.update`, keeps its binding through every other field.
- No change to `internal/service`, `internal/store`, `internal/core`, `internal/web` or `cmd`.
