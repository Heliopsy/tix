## Why

The browser's task heading counts the whole filtered selection ("12 to go, 3 done · across 4 projects · 2 held
by an agent · 5 waiting on other tasks") from an in-process tally. A remote reader such as tix-mobile only gets
`total` and the page it loaded, so a list paged 50 at a time says "12 to go" when 437 are open, which is the
bug the browser fixed in `waiting-on-deps`.

## What Changes

- `GET /api/v1/tasks` takes `tally=true` and answers the same tally the browser's heading reads, as `tally` on
  the page: tasks per project and status, tasks waiting on a dependency, tasks under a live claim.
- `internal/client` sends and reads it, so a remote caller gets what an in-process one does.
- No new operation and no new filter term: the tally is the shape of the answer, not a set of tasks. The
  command line and the terminal interface draw no heading that would use it.

## Impact

- `core.TaskPage.Tally`, `core.TaskTally` and `core.StateCount` gain JSON names; `wire.Page` carries `tally`.
- `api/openapi.yaml`, `docs/api.md`.
