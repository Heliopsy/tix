# Live updates keep the reader in place, and every task view filters by project

## Why

Under a burst of changes the task list, the board and the lanes were redrawn whole up to three
times a second. A wheel or a swipe in progress was scrolling a box that had just been replaced,
so scrolling snapped back; every scrollbar on the screen was new and blinked; every row replayed
its entrance; and a task added above the reader pushed what they were reading down. A pressed
button faded its label out over 120ms while its spinner was already drawn on top of it.

The board and the lanes could only be narrowed to a project by typing `project:KEY` into the
filter box. Nothing on screen offered it.

## What Changes

- A live refresh morphs the regions it redraws instead of replacing them: every row, card,
  column, lane and scrolling box still present stays the same element, only genuinely new rows
  are inserted, and an open disclosure stays open.
- Each scrolling box, sideways and down, stays on the row and column the reader was looking at.
  New rows above them raise a "N new, show" pill instead of moving the page.
- A listing redraws at most twice a second under a burst (500ms coalescing window, last wins).
- A pressed button is pending in the same frame: no transition, the spinner takes an icon's
  place beside a readable label or the place of a hidden label, the width never changes, and a
  second press is swallowed. A button with its own `hx-post` gets the same mark. The busy
  pointer on a form waits 300ms. Background refreshes never mark anything busy.
- The task list frame, board columns and lanes keep their scrollbar's room.
- The filter bar gains a project control shared by the list, the board and the lanes. It
  submits `project=KEY` (several keys comma separated), folded into the expression as
  `project:` terms like the deadline control. It survives the view switch, the pager, the live
  refresh, CSV export and a saved filter, and narrows the live subscription to those projects.

## Impact

- `internal/web/assets/live.js`, `busy.js`, `app.css`, `templates/tasks.html`, `tasks.go`,
  `live.go`, `savedfilters.go`, `bulk.go`, `mywork.go`, Bulgarian strings.
- No service method, route, migration or API change.
