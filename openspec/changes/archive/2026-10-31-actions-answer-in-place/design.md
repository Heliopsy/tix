# Design

## Declaring the swap on the element

The star form is one partial used on rows, lane cards and the task screen, so the swap target cannot
be written into the form. The element a form sits in says what its forms swap: `data-inplace` with
no value means the element itself by id (a row), `data-inplace="main.wrap"` names a selector (the
task screen's content). At submit, in the capture phase and so before htmx's own handler reads them,
feel.js writes `hx-target`, `hx-select`, `hx-swap="outerHTML show:none transition:false"`,
`hx-push-url="false"` and `hx-select-oob="#toast"` onto the form. The server is unchanged: it still
redirects to the listing, and htmx selects the one row out of it. A row that no longer matches the
filter comes back as nothing and leaves the list, which is what the tick already did.

In the task dialog the dialog's own body is the target, so nothing there declares `main.wrap`.

## Errors

live.js swaps any refusal so the error screen can explain itself. An in-place swap selects one row
out of that screen and there is none, so the row would vanish. feel.js cancels that swap for its own
forms, puts back a clone of the region taken before the optimistic change, and shows the refusal's
`.failure-message` in a toast.

## Undo

The redirect carries `flashundo`: `restore`, `priority:N`, or a route of states. The route is
`core.Routes` from the new state, filtered to the one ending in the old state, so a workflow with no
way back offers no Undo. Every place that builds an address from the request drops it, the same as
`flash`.

## Press, not hover

Fetching on hover and swapping the answer in by hand skipped htmx's history bookkeeping: htmx keeps
the path of the screen on show and saves the DOM under it before restoring, so Back saved the task
screen under the listing's address and restored it there. Starting the link's own click on
`mousedown` keeps htmx, the task dialog and history in charge, and takes the press-to-release time off.
The trusted click that follows within a second is swallowed whatever it lands on, since the next
screen may already be under the pointer.

## Scroll

htmx's history restores the window's scroll, and the task list scrolls inside `ul.tasklist`.
Positions of scrolled boxes in `main` are kept in `sessionStorage` under the path htmx is saving
(from the event, because on Back the address has already changed) and put back on restore.
