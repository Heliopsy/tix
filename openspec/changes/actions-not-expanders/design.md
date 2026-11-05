# Design

## One dialog mechanism

`actiondlg.go` carries which dialog a screen draws open in `view.Dialog`: the one `?dialog=ID` names, or
the one a refused submission came from. `refuseIn` draws the screen a submission came from again with a
refusal attached, through the screen's own GET handler, so the list under the dialog is the same list.
The templates use three partials: `dlg-open` (the button, a link without scripting), `dlg-start` and
`dlg-end` (the `<dialog>` frame with its title and close control). `assets/forms.js` opens the dialog
modally on a click, restores focus to the button on close, and turns a dialog drawn open in place into a
modal one on load and after every htmx swap.

A native modal `<dialog>` traps focus and closes on Escape without any script of ours. Without scripting
nothing can open it, so the opener is a link and the server draws the dialog open in place, which is
also what makes a refused submission work with scripting off.

## Why `?dialog=ID` and not `/new` routes

A query parameter on the screen's own route needs no new route per form, keeps the list beside the form,
and is ignored by everything else that reads the address. The parity rule that a browser route is chosen
by its path still holds: the parameter selects nothing on the server but which dialog is drawn open.

## Project settings get a route

They are a screen's worth of forms, and the board's live fragment redraws only the columns, so the board
and its settings were already separate in everything but the page. `GET /projects/{key}/settings` binds
`GetProject`, `ListWorkflows` and `GetHousekeeping`; the board no longer reads housekeeping.

## What stays a `<details>`

Toggles on one row or one field: a card's Move menu, the (i) tips, a task's Rename and Edit switches, its
Artifacts and History sections, a release's notes, a sync run's warnings, the extra fields inside Add with
details and an activity row's raw hops. The template guard lists them with a reason each.
