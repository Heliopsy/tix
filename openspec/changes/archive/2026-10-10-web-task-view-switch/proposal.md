# One task screen, drawn as a list or as a board

## Why

The browser has two screens over the same work. `/tasks` is the queue: every project at once, the
shared filter language, the deadline window, the sort, the column picker, the project visibility
choice and a cursor-paged listing. `/projects/{key}` is a board: one project, one workflow, columns,
drag to move -- and none of the task screen's controls, because it is a different screen with a
different handler.

So a reader who wants to see their filtered queue as columns has nowhere to go. They can open one
project's board and lose the filter, the sort and every other project, or they can stay on the list.
The thing they asked for -- *these* tasks, in columns -- was not expressible, and "go to a different
screen" is not an answer because the other screen answers a different question.

The obstacle is honest rather than incidental. A board needs columns and columns come from a
workflow, while the list cheerfully mixes projects. Two projects can only share a board if they agree
about their states, and agreement cannot be decided from the workflow's name: two projects can each
run a workflow called `default` whose states differ, and the state-category vocabulary has just
widened from three words to six, so two same-named workflows can now differ in ways that decide where
a task may go. Merging on the name would draw one set of columns over two different state machines
and offer a card a move its own project forbids -- a card that looks draggable to a column it can
never reach, with the refusal arriving from the service after the reader has already acted.

## What Changes

- The task screen gains a **view switch**, list or board, beside its filter bar. It is a per-browser
  preference in a cookie, `tix_task_view`, alongside the column picker and the project visibility
  choice it sits with, and defaults to the list.
- **Both views answer one selection.** The filter, the deadline window, the sort, the page size, the
  cursor and the project visibility choice are resolved once and then either listed or laid out in
  columns. Switching the view changes the drawing and never which tasks are on screen. One pager
  serves both.
- **Columns merge only across workflows that are genuinely the same state machine**: the same states
  in the same declared order, with the same labels, terminal flags and categories, and the same
  transitions in the same order with the same scope and comment requirements. Name, key and stored
  identity are not consulted, so two separately stored `default` workflows that agree do merge and
  two that disagree do not.
- **Every card's moves come from its own project's workflow**, resolved per card rather than from the
  definition the columns were drawn from, so no card is ever offered a transition its own project
  forbids. The board stays single-hop under drag and offers multi-hop routes only through the card's
  own Move control, exactly as the project board does: a drop names a column and nothing else.
- **When the selected projects do not agree, the board is refused and the list is drawn**, with a
  notice naming each distinct workflow and the projects running it, each one click from a board of
  its own with the reader's filter and sort intact. A board of whichever workflow came first would
  show a subset of the selected tasks without saying so.
- A task whose status is in none of the drawn columns is named in a notice rather than dropped from
  the board.
- A drag-and-drop move refreshes the board it is on, query string included. It re-fetched the path
  alone, which on the task screen would have answered with an unfiltered listing.

## Impact

- `internal/web`: the preference (`taskview.go`), the merge rule and the board assembly
  (`taskboard.go`), the task handler, `templates/tasks.html`, `assets/app.css`, `assets/live.js`.
- One new browser route, `POST /taskview`, which stores a display preference and touches no service
  operation, like every other preference route.
- `docs/web-ui.md`.

Not changed, and deliberately: the project board at `/projects/{key}`, which remains the single
project's own screen with its settings and danger zone; the service layer, which gains nothing and
is asked nothing new; and the multi-hop decision, which stays where
`multi-hop-transitions` left it.
