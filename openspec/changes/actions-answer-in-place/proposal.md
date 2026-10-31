# Actions answer in place

## Why

Measured with a browser on a seeded scratch store, every action on a list row except the tick
posted, followed the redirect to the whole listing and swapped the body. The list scrolls inside its
own box, so it went back to the first row; focus fell to the body; nothing on screen changed until
the round trip ended; and the live refresh that followed the action replaced the listing again,
which reset the scroll a second time. The task screen did the
same for every star, move, tag and comment. Back from a task returned to the top of the list.
Switching between list, board and lanes reloaded the page with its stylesheet and scripts. "?"
stopped opening the shortcuts after the first boosted navigation, because the overlay was looked up
once at load.

## What Changes

- A list row's star, status and priority forms swap that row only; the task screen's forms swap the
  page content, not the sidebar. Declared with `data-inplace` on the element, read by a new
  `assets/feel.js`; without JavaScript the forms post and redirect as before.
- The star toggles at once, a moved row's pill and a changed priority show the chosen value at once,
  and the affected part fades slightly while the request is out. A refusal or a failed request puts
  the previous state back and says why in a toast. Focus returns to the control that was used.
- The toast comes with the swap (`#toast`, always present), leaves on its own after a few seconds
  unless pointed at or focused, and offers Undo for a move whose way back the workflow allows, a
  priority change and a soft delete. Undo is a plain form.
- Live refreshes also wait for a row drawn ahead of its answer, and keep the scroll position of boxes
  inside the region. Waiting for requests and open menus, re-processing the markup and putting focus
  back came with the lane card change; focus now goes through one rule (`tix.focusMemo`) for live
  updates, lane cards and in-place actions alike, with a menu choice counted as its opener.
- Links into tasks and listings start on the mouse press rather than the release. Boosted
  navigations cross-fade where the browser has view transitions, except for a reader who asked for
  reduced motion. Back restores the scroll position of the task list.
- The task filter applies as the reader types (300ms after the last key) and when a select changes;
  Apply stays for a browser without JavaScript.
- The view switch is boosted and marks the chosen view on press.
- The shortcuts overlay is looked up when used, and the sidebar has a Keyboard shortcuts button.
- Floating menus and the toast cast a lighter shadow; menus drop in over 120ms.

## Impact

- `internal/web`: `assets/feel.js` (new), `assets/live.js`, `assets/decide.js`, `assets/shortcuts.js`,
  `assets/app.css`, `templates/layout.html`, `templates/task.html`, `templates/tasks.html`,
  `templates/partials.html`, `render.go`, `taskdetail.go`, `tasks.go`, `live.go`, `taskboard.go`,
  `i18n.go`; `internal/i18n/bg_feel.go`.
- No route, service method or capability changes.
