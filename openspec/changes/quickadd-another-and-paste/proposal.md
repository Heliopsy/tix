# Add another, and quick add from the clipboard

## Why

The tix app's quick add sheet has an "Add another" switch that keeps it open for the next line, and a
key (ctrl+shift+v) that starts a quick add from the clipboard. On the web, adding a task returned to
the listing (or opened the task, with Open a new task on) with the box no longer focused, so typing
five tasks in a row meant pressing `n` or clicking the box before each one, and pasting a line from
elsewhere meant focusing the box first.

## What Changes

- An **Add another** checkbox beside the quick-add box. With it ticked, an add returns to the listing
  with the box focused and empty, whatever Open a new task says. The choice is posted with the form
  and remembered in this browser (`tix_add_another`), so it works with scripting off.
- A **Quick add from the clipboard** button beside the box, and `ctrl+shift+v` (command+shift+v on a
  Mac) on the task screens in every key scheme. It puts the clipboard's first non-empty line in the box
  and focuses it without adding; an empty or unreadable clipboard says "The clipboard holds no text."
  The button is drawn hidden and revealed only where the browser has the Clipboard API.
- The keyboard chord reader names ctrl+shift and command+shift letter chords as such, so
  `ctrl+shift+v` is not read as `ctrl+v`.

## Capabilities

### Modified Capabilities

- `web-tasks`: Add another and the clipboard paste on the quick-add row, and the new shortcut.

## Impact

`internal/web/tasks.go`, `internal/web/quickanother.go` (new), `internal/web/shortcuts.go`,
`internal/web/templates/tasks.html`, `internal/web/templates/layout.html` (one script tag),
`internal/web/assets/quickadd.js` (new), `decide.js`, `shortcuts.js`, `icons.svg`, a new stylesheet
part, Bulgarian strings, `docs/web-ui.md`.
