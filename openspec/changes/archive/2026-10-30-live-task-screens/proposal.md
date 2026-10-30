# The task screens update themselves

## Why

`grep -rl data-feed internal/web/templates/` answers `activity.html` and
nothing else. The task list, the board and one task's own screen carry no live
hook at all, and `assets/live.js` binds only the element carrying `data-feed`,
so none of them reflect a change made anywhere else: not from the terminal,
not from another tab, not from an agent claiming a task.

Reported: a task moved from todo to done in the SSH TUI. The open web task
screen went on saying todo. A reload showed the change, so the data was right
and only the screen was stale.

The mechanism the feed uses is also thinner than it looks, and multiplying it
across three more screens would have multiplied the holes:

- **No reconnect.** The socket has `open` and `message` listeners and nothing
  else. A laptop that sleeps, a proxy that idles the connection out, a server
  that restarts, leaves the page permanently silent and says nothing.
- **No resume.** It subscribes with no `since_seq`, although the server
  implements gap-free replay for exactly this case, so every change during a
  gap is lost.
- **No fallback.** Where a socket cannot be established, nothing updates and
  nothing says so.
- **Trailing-edge debounce.** A single status change waited out the full
  300ms quiet period for nothing to coalesce, and a burst pushed the render
  further away with each event rather than closer, with no bound.

A screen that has silently stopped updating looks exactly like one that is up
to date. That is the defect, and a live mechanism that can fail silently is
the same defect one layer up.

## What Changes

**Server-rendered fragments, swapped by the client.** Each screen renders a
binding element (`partials.html` "live-screen") naming where to re-fetch, what
to subscribe to, and which regions the answer replaces. Each fragment route
(`/tasks/live`, `/projects/{key}/live`, `/tasks/{ref}/live`) runs the very
template blocks the full page used, so there is no second copy of a row, a
card or a status pill in JavaScript, and no client-side model of task state.
Nothing rendered travels over the socket.

**The refresh preserves the screen's state.** The fragment URL carries the
request's own query string -- filter, deadline window, sort, cursor, page size
-- and the preferences that are cookies ride along on the fetch, so the
list/board switch, the column picker and the project visibility choice are
preserved too.

**The refresh never lands on work in progress.** A card held in a drag, or a
field inside a replaced region that has been typed into or holds the caret,
defers the update rather than discarding it; it lands as soon as the region is
idle.

**One connection mechanism for all four screens**, the feed included:
reconnect with jittered exponential backoff, resume with `since_seq`, an HTTP
fallback poll when no socket can be opened, and an on-screen notice whenever
the view is not live.

**Leading-edge coalescing.** `tix.debouncer` is replaced by `tix.coalescer`:
the first event renders at once, the rest of a burst collapses into one more
render, and a maximum wait stops a continuous stream starving the trailing
one.

## Impact

- `internal/web/live.go` (new): the binding, the subscription filters, the
  fragment writer, and the screen's own URL for a row action.
- `internal/web/tasks.go`, `taskdetail.go`, `projects.go`: each screen's view
  is built by a function the page handler and the fragment handler share.
- `internal/web/routes.go`: three fragment routes.
- `internal/web/templates/`: `tasks.html`, `board.html`, `task.html` grow
  named blocks for the regions a refresh replaces; `partials.html` grows the
  binding and the stale notice; `activity.html` renders the notice.
- `internal/web/assets/live.js`, `decide.js`, `app.css`.

**What does not change.** The WebSocket protocol, `internal/httpapi`,
`internal/service` and `internal/outbox` are untouched: the stream already
carries everything these screens need, including the `since_seq` resume and
the subscription filters.
