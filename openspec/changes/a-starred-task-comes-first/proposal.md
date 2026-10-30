# A starred task comes first

## Why

A reader working a shared queue has a handful of tasks they care about right now, and the
listing has no way to hold them. Sorting by priority or by deadline answers a question about
the task; "the ones I am watching" is a question about the reader. The request was a star
button that keeps a task above the rest.

## What Changes

- A reader can star and unstar a task. A star is **personal**: it belongs to the actor who set
  it, within one tenant, so two people sharing a queue star different things and neither
  reorders the other's screen. It is stored server-side, so it follows the reader across the
  browser, the command line and the terminal interface.
- Starring and unstarring are mutations: each writes its row, an audit entry and an outbox
  event (`task.starred`, `task.unstarred`) in one transaction. Both are idempotent; a no-op
  writes nothing.
- Every task listing puts the reader's starred tasks first and keeps the existing order inside
  each group, under every sort. The listing stays keyset-paginated: the starred flag becomes
  the leading sort key and travels in the cursor.
- The filter language gains `is:starred` / `is:unstarred` (and `starred:true|false`); the
  command line gains `--starred`, the HTTP API `starred=true|false`.
- A task read (show and list) carries `starred` for the reader.
- Surfaces: `tix task star` / `tix task unstar`, `PUT` / `DELETE /v1/tasks/{ref}/star`, a star
  button on list rows, board cards and the task screen, and the `*` key in the terminal
  interface.
- Hard deleting a task removes every star on it. Stars do not travel in snapshots.

## Impact

- New table `task_stars` (migration 0014), new store methods on both engines, a leading key in
  the task listing's keyset.
- `core.Task.Starred`, `core.TaskFilter.Starred`, `core.Cursor.Starred`, two event types, one
  authz action, two `Service` methods with registry entries on every surface.
- No change for a reader who stars nothing: their listings are byte-for-byte the order they
  were, and snapshots are unchanged.
