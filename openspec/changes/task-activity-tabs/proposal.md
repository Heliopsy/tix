# Task activity tabs

## Why

A task's History and Artifacts sat folded at the bottom of the screen under their own headings, so
nobody saw them unless they knew to open them, and Comments sat between them. The folds were the last
page sections still in an expander. In the task panel the due date's hint was cut off at "tomorrow, +3d
or 2026-" because the column is narrow. And `tix tenant user ls` printed each user as a Go struct in
one USER column.

## What Changes

- Comments, History and Artifacts become a tab strip under Dependencies, each tab with its count, zero
  shown muted. Arrow keys, Home and End move between tabs. The last tab chosen is remembered per
  browser. `#comments`, `#history`, `#artifacts`, a record's own fragment, an older page of history and
  a raw entry in the address open the tab holding them. Without scripting all three panels show, one
  under another with their headings. A live update redraws the lists and counts and keeps the tab.
- The task dialog draws the same tabs.
- The due field in the task panel shows the date in the reader's format as its hint; the words and
  offsets stay in its (i) tip. The add form keeps the whole hint.
- `tix tenant user ls` draws ID, EMAIL, NAME, ROLE, GLOBAL ADMIN, CREATED and DISABLED columns.

## Capabilities

### Modified Capabilities

- `web-ui`: the task screen's activity is tabs, and a linked record on a hidden tab is shown.
- `cli`: `tix tenant user ls` renders a readable table.

## Impact

`internal/web/templates/task.html`, `internal/web/assets/tasktabs.js`, `assets/copy.js`, `app.css`,
`internal/web/live.go`, `internal/web/taskdetail.go`, `internal/output/table.go`, `docs/web-ui.md`.
