# Design

- **A third position, not automatic.** Every way a task opens on the web is a per-browser choice the reader
  makes on Settings, and Page is the default because that is what the interface did first. Making the panel
  automatic on wide windows would change what a click does for every existing reader. Panel is a third
  segment of the same control, beside Page and Dialog.
- **The dialog, drawn beside.** The panel is `#taskdlg`, the element Dialog mode already draws, opened with
  `show()` instead of `showModal()` and fixed to the right of the window. Everything the dialog solved is
  reused unchanged: the `X-Tix-Dialog` fragment with prefixed ids, the history entry and its popstate rules,
  a save's redirect drawn in place, a delete leaving for the listing, a refusal shown above the task.
- **Breakpoint.** 1100px: the sidebar column starts at 900px and is 224px by default, so 1100px leaves the
  list a usable width beside a 360px panel. The app's 840px is a window without a 224px sidebar.
  Below it Panel falls back to the dialog, the nearest thing that keeps the list. A window resized across the
  breakpoint moves an open task between the two.
- **Width.** Default 460px, at least 360px, at most 720px and never more than 60% of the room beside the
  sidebar: the app's `detailMinWidth`, `detailMaxWidth`, `detailDefaultWidth` and `detailMaxFor`. Arrow keys
  move 16px, 64px with Shift; Home and End go to the bounds; Enter and a double click reset. Left widens the
  panel, since the divider moves left.
- **Kept like the sidebar.** The sidebar's width lives in a cookie, so the panel's does too: `tix_pane`,
  `w=NNN`, a year, removed at the default. The server never reads it, since the panel is only drawn by the
  script; it is a cookie so both widths live in one place.
- **History.** Opening pushes the task's address, as the dialog does; a task followed from the list while the
  panel is open replaces the entry, so Back closes the panel rather than walking every task looked at. A
  reload of that address draws the task's own page. A boosted navigation from the list closes the panel and
  returns its entry to the list first, so the history snapshot never holds an open panel.
- **Keyboard.** Escape inside the panel closes it unless a menu is open or something inside took the key.
  The global shortcuts act on the list while focus is outside the panel and stand down inside it.
- **Limits.** The task in the panel is not live: as in the dialog, a save made in it refreshes the list, but a
  change made elsewhere reaches the panel only when it is reopened. The list stays live.
