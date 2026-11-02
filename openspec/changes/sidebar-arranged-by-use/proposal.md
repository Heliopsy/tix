# The sidebar arranged by use

## Why

The sidebar grew one entry at a time and its order shows it. My work, the screen a reader opens most,
was drawn indented under Trash as if it were part of it. Trash, opened rarely, sat second. Configure
and Data were two folded groups for the same audience. The foot stacked four separate blocks (the
handle, Settings with the scheme switch, the shortcuts hint, Sign out) under the version, the tallest
strip of the column for the things in it used least. There was no way to reach a project's board from
the sidebar.

## What Changes

- Order by use: the tenant's name with the rail toggle, Search, then Inbox, My work, Tasks, Projects,
  Activity and Statistics. My work is a top-level entry with a badge counting the reader's overdue open
  tasks, refreshed by the same live stream as the unread badge.
- Pinned projects under Projects, each with its icon and colour, linking to its board and named the
  way the reader asked projects to be named. Pinning is the per-reader star a project lane already
  has; the projects screen gets a Pin to sidebar switch on each row.
- Saved filters and Recently viewed follow as the reader's own views, folding as before.
- One Admin group replaces Configure and Data and takes Trash, folded by default. A browser that had
  either old group open gets Admin open.
- One account menu at the foot: avatar and handle opening Settings, the colour scheme, Keyboard
  shortcuts and Sign out. The version link sits under it.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: the sidebar's order, pinned projects, the Admin group and the account menu.

## Impact

`internal/web` templates, `sidebar.go`, `projects.go`, `inbox.go`, `assets/sidebar.js`,
`assets/live.js`, `assets/decide.js`, `app.css`, the Bulgarian catalogue and `docs/web-ui.md`. No new
service method, route, cookie or migration: pinning reuses group preferences, and the overdue count is
the overdue filter the task screen already answers.
