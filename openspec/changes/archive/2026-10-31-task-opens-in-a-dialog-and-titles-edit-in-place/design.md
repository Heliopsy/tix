# Design

## Two cookies, same shape as the others

`tix_task_open` and `tix_title_click` copy `tix_drag_move`: absent is the default, one value turns
the other behaviour on (`dialog`, `edit`), anything else reads as the default. Both are switches on
Settings, in Listings, because they change how a listing behaves rather than how anything looks. The
switch's state word names the current choice (Page or Dialog, Opens the task or Edits the title), as
Advanced screens already says Shown or Hidden.

## The dialog is htmx into a native `<dialog>`

The layout renders an empty `<dialog id="taskdlg">` on every screen except the task screen itself,
only when the preference is Dialog. Its attributes make every request from inside it target its body,
not push a history entry, and carry `X-Tix-Dialog: 1`. `assets/taskopen.js` intercepts a plain
left click on any link to `/tasks/{ref}` in the capture phase, ahead of the boost, and asks htmx for
the task into the dialog with the same header.

The server answers that header with the `task-dialog` block of `task.html` instead of the layout: a
wrapper carrying `data-task-dialog`, the flash, and the same `content` block the page draws. So there
is one task screen, drawn two ways, and no second copy of any part of it. The wrapper is what the
script checks a response for: a save that redirects somewhere other than the task (deleting it goes
back to the listing) answers with a whole page, and the script closes the dialog and navigates to it
rather than drawing a page inside the dialog. A refusal (4xx) keeps the task in the dialog and shows
the error screen's message above it.

Every boosted form in the task screen posts and redirects as it does on the page. The redirected GET
keeps the request's own headers, so the answer is the fragment again and the dialog redraws in place.
The listing under it is refreshed by the live binding it already has: the save writes an event, and
the script also pokes the binding directly (`tix:refresh`), so a page without a socket catches up too.

Inside the dialog the task screen drops its own live binding (`#live-screen` is the listing's, and a
second element with that id would bind the wrong one) and prefixes the ids that collide with a
listing's (`flow-`, `prio-`, `body`, `priority`, `repeat`), so a popover button in the dialog opens
the dialog's menu and not the row's behind it.

## History

Opening pushes the task's address with `{tixDialog: true, over: <the listing's address>}`. Closing by
any means goes Back. A popstate that leaves a dialog entry closes the dialog and is stopped before
htmx sees it, so htmx does not restore a snapshot of the listing over the live one. Forward onto a
dialog entry reopens the dialog when the page under it is the one it was opened over, and reloads the
address otherwise, which draws the full task page. A link to another task inside the dialog replaces
the entry rather than adding one, so Back always closes. Any other link inside the dialog replaces
the entry with the listing's address and navigates the whole page.

## Focus

`showModal()` makes the rest of the page inert, which is the focus trap. On close the script focuses
the element that opened it, or, when a live refresh has replaced that row, the same task's row.

## Editing a title in place

The title link carries `data-edit-title`, the task's version and its reference only when the
preference is Edits the title, and the open link is drawn beside it. The script turns the link into a
text field. A save posts `csrf_token`, `version` and `title` to the task's existing edit address with
`redirect: "manual"`, so success is the redirect and nothing is downloaded. A refusal's body is the
error screen, whose message and hint the script shows under the field. The live binding treats the
focused field as busy, so a refresh cannot replace the row while it is being edited; a successful
save pokes the binding, which brings the new version.

For a save naming only the title to leave the description and assignee alone, `updateTask` now sets
those two only when the form names them, the rule it already applies to custom fields and the repeat
rule. Both forms on the task screen name them, so nothing a browser submits today changes meaning.

## What was not done

The dialog does not keep itself live while open. Its own binding would compete with the listing's for
one `#live-screen`, and a second socket per page for a view that lasts a few seconds is not worth it.
A save from the dialog redraws it, and reopening shows the current state.
