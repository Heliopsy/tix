# Every screen state has a way back

## Why

A user on 0.43.0 reported that once they tried a mapping there was no way back. Walking the flows
that take a mapping found it on the external sync screen: a run refused for its mapping file landed on
the error screen, whose only way back pointed at `/sync/run`. That address answers POST alone, so the
link drew "Not found" with nothing on it but the task list. Every other action without a screen of
its own (`/sync/sources/delete`, `/transfer/import`, `/bundles/import` and the rest) had the same
trap.

A run that worked was hard to leave too. Its result was drawn under the explanation at the foot of the
screen, at the address `/sync/run`, so it was off screen after pressing Run, and reloading or coming
back to it either posted the run again or found no page.

## What Changes

- The error screen leads a failed action with no screen of its own back to the same-origin screen it
  was posted from, and to the task list when there is none.
- A sync run's result, and a run refused for its source or mapping file, are drawn at the top of the
  sync screen with a "Back to the sources" link, and the browser is asked to show `/sync` rather
  than the action's address.
- A refused dialog form drawn for a different address than it was posted to asks the browser to show
  that screen with the dialog open, so reloading it or coming back to it does not post again.
- A run's answer is scrolled into view after the swap. On a phone the Run button sits far down the
  list, and the swap kept that place, so the answer drawn at the top was never seen.
- The deletion screen's mapping preview names the target the picker shows. A target named in another
  case than its key left the picker on the first project while the table described another.
- The project deletion screen's way back to the board uses the same back link as the project settings
  screen.
- `AGENTS.md` gains the rule, and a guard walks the states of these flows.

## Impact

- `internal/web`: `render.go`, `sync.go`, `actiondlg.go`, `templates/sync.html`,
  `templates/projectdelete.html`, `projectdelete.go`, `assets/app.css`, `assets/decide.js`,
  `assets/copy.js`
- `internal/i18n`: three Bulgarian strings
