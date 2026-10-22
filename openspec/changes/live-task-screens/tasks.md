# Tasks

## 1. The screens carry a live binding

- [x] 1.1 `internal/web/live.go`: `liveScreen`, the per-screen subscription filters, `fragmentHref`
      carrying the request's query, `screenHref` for a row action's return, and `renderFragment`,
      which executes one named block of the screen's own template set
- [x] 1.2 `internal/web/templates/partials.html`: the "live-screen" binding element and the
      "live-stale" notice; `activity.html` renders the notice too
- [x] 1.3 `internal/web/routes.go`: `/tasks/live`, `/tasks/{ref}/live`, `/projects/{key}/live`

## 2. One build per screen, two renderings

- [x] 2.1 `tasks.go`: `buildTasksView` shared by `showTasks` and `showTasksLive`; `selfURL` replaced
      by `screenHref`, so a row that arrived live returns to the screen and not to the fragment
- [x] 2.2 `taskdetail.go`: `buildTaskView` shared by `showTask` and `showTaskLive`
- [x] 2.3 `projects.go`: `buildBoardView` shared by `showBoard` and `showBoardLive`
- [x] 2.4 `templates/tasks.html`, `board.html`, `task.html`: the replaced regions become named
      blocks the page and the fragment both run

## 3. The connection

- [x] 3.1 `assets/decide.js`: `coalescer` replaces `debouncer` (leading edge, bounded trailing),
      `backoffDelay`, `resumeRefused`, `staleNotice`, `wantsEvent`, `fieldDirty`, `regionBusy`
- [x] 3.2 `assets/live.js`: one `liveStream` behind the feed and the screen binding -- reconnect
      with backoff, `since_seq` resume, the 15s fallback poll, the stale notice, the visibility
      rule, and the defer-never-drop rule for a region the reader is working in
- [x] 3.3 `assets/app.css`: the notice's styling, in one block at the end of the file

## 4. Tests

- [x] 4.1 `internal/web/livescreen_test.go`: every screen carries a binding; the fragment keeps the
      filter, the sort, the cursor, the board switch and the hidden projects; the fragment is only
      the regions it replaces; a transition made elsewhere reaches the task fragment; the
      subscription names only what the screen draws; a refreshed row is byte-identical to the
      page's; every screen can say it is not live; the coalescing windows are 300 and 150
- [x] 4.2 `internal/web/jstest/live.test.mjs`: subscribe filter, leading-edge render, region swap,
      a neighbour's event ignored, a drag and a dirty field each deferring and then landing,
      reconnect with `since_seq`, a refused resume, the fallback poll and notice, the hidden-page
      rule
- [x] 4.3 `internal/web/jstest/decide.test.mjs`: the coalescer's leading edge, its grouping, its
      bounded wait and its re-arm; the backoff curve and its jitter band; `resumeRefused`;
      `staleNotice`; `wantsEvent`; `fieldDirty`

## 5. Gates

- [x] 5.1 Watched the guards fail: the binding removed from `tasks.html`; `fragmentHref` returning
      the bare route; the task fragment rendering the whole screen; the `since_seq` line removed;
      the busy check removed; the coalescer's leading edge removed. Each restored and verified
- [x] 5.2 `just check` exit 0
