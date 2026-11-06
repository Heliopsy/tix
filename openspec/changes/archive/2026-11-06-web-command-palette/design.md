# Design

## What is embedded and what is asked for

Screens and actions are fixed per page: the sidebar's destinations (only the Configure and Data ones when
Advanced screens is on, as in the sidebar), the sections of Settings and the actions. `palette.go` builds
them with labels already translated and the layout embeds them as `tix-palette-data`, next to the shortcut
table. Opening the palette draws them with no request.

Projects change rarely and are cheap to list, but listing them on every page render would add a query to
every screen for a dialog most renders never open. They are read from `/palette/projects` when the palette
opens and kept for a minute.

Tasks are the one group that needs the server for every query. Nothing is sent until something is typed;
then one request goes out 150ms after the last keystroke, an earlier one in flight is aborted, and an
answer to anything but the current query is dropped.

## No new capability

A palette search is the filter bar's question with a smaller page: `ListTasks` under `query.Parse`, so
`is:starred deploy` works as it does in the filter bar and a half-typed term falls back to plain words
instead of refusing. A query shaped like a ref is also looked up with `GetTask` and put first, since the
store's text search covers titles and bodies and not refs. The page is ten tasks, most recently changed
first, keyset like every other listing. Over the API the same answer is `GET /api/v1/tasks?q=` with a
limit, so there is no new service method, registry entry or API route. Both palette routes are web
bindings of existing operations and run behind the same session and tenant scope as every screen.

## Order

With nothing typed the order is Recent, Actions, Screens, Projects. Once something is typed, groups
are ordered by their best match, so "board" puts Show tasks as a board ahead of the Keyboard section of
Settings, and the selection follows the option the reader had rather than its index, so tasks arriving
150ms later never change what Enter does.

## Keys

ctrl+k matches the TUI's palette key and is free in all three web schemes; meta+k is the Mac's. Both are
an action in the shortcut table, so the "?" overlay and the Settings table list them without extra code.
The shortcut layer ignores keys while typing, but the palette should open from inside a field too, so
`palette.js` listens for the chord itself and stands down when the shortcut layer has already claimed it.
`comboOf` now names command as a modifier; before, command+g reached the "top" shortcut.

## Choosing

Each action drives an existing control: the quick-add field and its project select, the filter field
(opening its popover on a phone), the New project sheet, the view switch or a post to `/taskview`, the
colour scheme switch, the help overlay, the sign-out form. When the current page lacks the control (New
task from Activity), the palette goes to the page that has it and finishes there, carried in
`sessionStorage`.

A task goes through `tix.openTaskHref` from `taskopen.js`, which opens it in the dialog when the page has
one, and returns focus to whatever had it before the palette opened. Without the dialog the browser goes
to the task's page.

## Recent

The palette keeps the last eight things chosen in `localStorage`, per browser like the other view state,
and lifts them: first, in order, with nothing typed; a bonus to their score otherwise. When the page
offers `tix.recentTasks()` (the sidebar's recently viewed tasks) those come first. Without it the palette
only has its own list.

## Accessibility

The input is a combobox controlling one listbox; groups are `role="group"` with a name, options carry
`aria-selected`, and the input points at the selected one with `aria-activedescendant`, so focus never
leaves the input. A status line announces "Searching tasks", "No matches" or a failed search. Tab is kept
inside the dialog, Escape is the dialog's own, and focus returns to the opener. On a coarse pointer every
option is 44px tall, and under 600px the dialog fills the screen.
