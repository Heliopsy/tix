# Adding a task stays on the list, back links go back, and the task fields are chosen in View

## Why

Three reports against v0.25.0.

"Back to the dashboard" on the error screen pointed at `/`, which is not a screen: it redirects to
the projects list, so a reader who wanted their task list landed on projects. The error screen's
other link did the same for any failed page, because a failed GET answered `/` as where the reader
had been. The task screen's own back link held the listing only for the first view: every save on
that screen redirects to the bare task address, so after adding a tag or a comment "Back to the list"
went to the unfiltered list.

Adding a task opened it. Most adds are a list being written down, one line after another, and each
one threw the reader off the list. The preference existed but defaulted to opening.

The View menu chose columns for the list but not the fields readers asked for, recurrence and the
star, and the board and lane cards ignored the choice entirely. The repeat marker was a bare glyph
whose rule was only in a hover. The task screen said nothing about when the next occurrence would
fall due, and the star state was a shape with no words.

## What Changes

- The error screen's links name where they go. A failed page goes back to the page that linked to it
  when that was one of ours, and otherwise to the task list. Nothing links `/`.
- The task screen's back link is named after its destination ("Back to tasks", "Back to the infra
  board") and keeps the listing across saves: a link without `next` is answered from the referring
  page, and every redirect back to the task screen carries the `next` it was opened with. Deleting a
  task returns to that listing too.
- Adding a task stays on the listing by default, for the one-line box and for Add with details. The
  toast says "task created" and links the new task, and its row or card is marked. A browser that
  turned "Open a new task" on keeps it.
- View offers two more task fields, Star and Repeats, both on by default. Repeats draws the glyph and
  the rule. Board and lane cards respect Star, Status (lanes), Priority, Tags, Repeats and Reference.
- The task screen's About card says whether the reader starred the task, names the task it is a
  subtask of, states the rule with the next occurrence's date (the one created, or the one finishing
  now would create), says when the rule has run out, and shows when the task started and finished.

## Impact

- `internal/web`: `render.go`, `session.go`, `tasks.go`, `taskdetail.go`, `columns.go`, `live.go`,
  templates `tasks.html`, `task.html`, `error.html`, `partials.html`, `app.css`.
- `docs/web-ui.md`, `demo/scene/browser.mjs` (the scene opened the new task from the redirect).
- No service, store or API change. No capability added.
