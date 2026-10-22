# Design

## The rule: the server renders, the client swaps

The feed already establishes it and it is kept: a notification says only that
something changed, and the screen answers by asking the server to render the
same thing again through the same template block. The alternative -- sending
rendered HTML in a socket frame, or keeping a client-side model of a task --
puts a second copy of the rendering rule, or of the domain, somewhere nothing
tests it. A fragment route is a GET like any other: it authorizes the same
way, reads the same cookies, and is readable in a browser.

## What a refresh replaces, and what it must not

| Screen | Regions replaced | Left alone |
| --- | --- | --- |
| Task list | summary line, listing (list or board) + pager | quick-add, filter bar, view switch, View panel |
| Project board | the columns | project settings, danger zone |
| One task | status line, comments, history | detail editor, move control, comment box, tags, danger zone |

Everything left alone is something the reader types into. The task screen's
edit form is also the task's optimistic lock: its `version` field is
deliberately not refreshed, so a save made against a version somebody else has
moved past is refused by the service, which is the correct answer and not a
silent overwrite under a version the page quietly replaced.

## Point three: defer, never drop

A refresh is held back while the reader is busy in a region it would replace:
a card in a drag (`.is-dragging`), the caret in a field inside the region, or
any field there changed away from what the server rendered into it.

Deferring beats the two alternatives. **Dropping** the update puts the screen
back in the defect this change exists to remove. **Refreshing everything
except the busy region** leaves a status pill disagreeing with the form under
it -- the same staleness, in a harder place to notice. The deferral costs only
the seconds until the reader stops: it re-checks every 500ms, which is a
comparison and not a request, and the update lands the moment the region is
idle. It is checked again when the fragment arrives, because a drag that began
while it was in flight is just as much work in progress as one that began
before it.

## Only the events that matter

The subscription filter is the screen's own: a project board names its
project, and every screen names the event types it can draw
(`task.*`, `tag.added`, `dependency.added`, plus `comment.added` and
`artifact.added` on a task screen). A webhook delivery or a sign-in never
reaches these pages, so it costs no fetch and no render.

The stream has no filter for a single record, so one task's screen narrows the
rest of the way itself, on the subject the event names -- `subject_id`, or
`payload.task_id` for a comment or an artifact. That is the only client-side
filtering, and it decides nothing about what is drawn.

## The connection

One implementation, four screens, because four copies of a reconnect is four
chances to get it wrong.

- **Backoff**: 0.5s, 1, 2, 4, 8, 16, 30, 30... each jittered over the lower
  half of its interval, so a server coming back up is not met by every open
  browser at the same instant. Reset on a successful open.
- **Hidden pages do not reconnect.** A sleeping laptop wakes with every open
  tab retrying at once, and a background tab has nothing to keep current.
  Returning to the page connects immediately and re-fetches, which is the
  sleep case and by far the common one.
- **Resume**: the highest `seq` seen is sent as `since_seq` on every
  resubscribe, and the server replays the gap. Nothing is inferred from the
  replayed events except that something changed; the fragment is still the
  answer.
- **A cursor the server will not honour** -- retired out of the log -- is
  answered by dropping the cursor, subscribing from now, and re-fetching the
  fragment whole. A full refresh is correct however much was missed, which is
  the only honest answer at that point.
- **Fallback**: where no socket can be opened, the same fragment route is
  polled every 15 seconds, and the poll stops the moment a socket opens. Slow
  on purpose: it is there so a reader behind an intermediary that eats
  WebSockets still sees changes within a quarter of a minute, not so that it
  becomes the normal path.
- **The notice**: whenever the stream is not live, a small line says the view
  may be out of date and whether it is reconnecting or polling. Hidden
  otherwise. A screen that has quietly stopped updating is indistinguishable
  from one that is current, which is the whole defect.

## Coalescing, and why the old debounce was wrong

`tix.debouncer` was trailing-edge only. A single status change -- the reported
case -- waited the full 300ms for nothing to coalesce, and a burst restarted
the timer on every hop, so the render moved further away with each event and
under sustained change could be starved indefinitely.

`tix.coalescer` runs on the leading edge, then coalesces the rest of the
window into one more run, with a maximum wait of 1s so a continuous stream
still redraws on a fixed beat. After a trailing run the window reopens empty,
so two renders never land back to back.

The windows: **300ms for a listing**, where a burst is routine -- a multi-hop
transition writes a hop at a time, an agent sweep touches a page of rows --
and a re-render is fifty rows nobody is watching individually. **150ms for one
task's screen**, where a burst is a handful of hops at most and the window is
the whole of the delay a reader perceives between the change and the pill
changing. Neither is the latency of the first event, which is now zero.

`tix.debouncer` is removed rather than kept beside the new function: it had no
remaining caller, and an unused trailing-edge debounce beside a leading-edge
coalescer is an invitation to pick the wrong one.

## What was not done

The server half of the latency is the outbox tailer's 250ms ticker and the
unused `pg_notify` path. Both are out of this change's scope and are being
fixed separately.
