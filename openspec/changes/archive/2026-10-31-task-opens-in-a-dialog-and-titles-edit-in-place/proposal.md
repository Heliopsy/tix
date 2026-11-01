# A task can open in a dialog, and a title can be edited where it is listed

## Why

Opening a task from a listing leaves the listing. Coming back is a page load, the scroll position is
whatever the browser restores, and checking three tasks in a row is three round trips away from the
list and three back. Some readers would rather look at a task over the list they are working through.

Renaming a task means opening it, finding the title field, saving and going back. On a list being
tidied up that is most of the work, for a change of a few characters.

Both are ways of working rather than fixes, so both are preferences, and the default for each is what
the interface does today.

## What Changes

- A new per-browser preference, **Task opens in** (`tix_task_open`): Page, the default, or Dialog.
  In Dialog, following a task link from a listing, a board, the lanes, the activity feed, search
  results or the toast after adding opens the task screen in a modal dialog over the page. The
  address changes to the task's, so a reload or a copied link opens the full page and Back closes the
  dialog. Escape, the Close button and a click on the backdrop close it, and focus returns to what
  opened it. Every save made in the dialog redraws the dialog and refreshes the listing under it. A
  modified click, a middle click and a browser without scripting open the full page as before. On a
  phone the dialog fills the screen. The task screen's back link reads Close in the dialog.
- The server answers a task screen request that carries `X-Tix-Dialog: 1` with the task screen's
  content alone, without the layout around it.
- A new per-browser preference, **Clicking a title** (`tix_title_click`): Opens the task, the default,
  or Edits the title. In Edits the title, a click on a task's title in the list, on a board card or a
  lane card turns it into a text field: Enter or leaving the field saves, Escape cancels, and a
  refusal is shown under the field. A small open link beside the title still opens the task, through
  the first preference. Enter or F2 on a selected row edits its title. Without scripting a title is
  a link to the task.
- Saving the task screen's edit form leaves the description and assignee alone when the submission
  does not name them, as it already does for custom fields, so the title can be saved on its own.
- Both preferences are switches in the Listings section of Settings.

## Impact

- `internal/web`: `render.go`, `session.go`, `prefs.go`, `routes.go`, `taskdetail.go`, `i18n.go`,
  templates `layout.html`, `partials.html`, `task.html`, `tasks.html`, `board.html`,
  `settings.html`, assets `taskopen.js` (new), `decide.js`, `live.js`, `shortcuts.js`, `app.css`,
  `icons.svg`.
- `internal/i18n`: Bulgarian entries for every new text.
- `docs/web-ui.md`.
- No service, store or API change. No capability added: the title edit uses the existing
  `UpdateTask` binding.
