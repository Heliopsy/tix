# Select mode

## Why

Every task row drew two tick-like controls side by side: a square checkbox that selects the row for
bulk actions and a circle that marks it done. Readers kept pressing one when they meant the other.
Selecting is the rarer job, so it should be something you ask for, not something every row carries.

## What Changes

- By default a row shows only the round done control. The selection squares are gone.
- A Select toggle in the task screen's toolbar (list, board and lanes) turns on select mode. It is a
  link to `?select=1` without scripting and a pressed/unpressed button with it.
- Select mode can also be entered by pressing `x` on a focused row, Shift-click or Ctrl/Cmd-click on
  a row's empty area, and a long press (about 500ms, without scrolling) on a touch screen.
- In select mode every row, board card and lane card shows a square checkbox labelled "Select
  KEY-12", the bulk bar shows with "N selected", and the done circles stay but are dimmed.
- Esc, Cancel in the bulk bar, or unselecting the last selected task leaves the mode.
- Keys: `x` toggles the focused row, Shift+Up/Down extend the selection, `d` marks the focused task
  done. `e` was asked for, but it is already Edit title in the default and emacs schemes and mirrors
  the terminal's edit key, so done takes `d`. The `?` overlay lists all three.
- The board and the lanes get bulk selection, which only the list had.
- `select` is kept in the address by the view switch, the pager, the filter form, Clear and the lanes
  control, and live refreshes and row swaps keep the mode and the selection.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-tasks`: row selection becomes a mode, reaches the board and lanes, and the bulk bar shows
  only in that mode.

## Impact

`internal/web` templates, `assets/select.js`, `assets/shortcuts.js`, `assets/decide.js` (Shift on
named keys), `assets/css/470-bulk-select.css`, the shortcut table in `shortcuts.go`, the Bulgarian
catalog, `docs/web-ui.md` and the screenshot manifest. No service, HTTP or CLI change: the bulk
route is unchanged.
