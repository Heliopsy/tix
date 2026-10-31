# Design

## One menu, inside the card

The menu is a popover inside the card it acts on, with an id per task, like the badge and priority
menus. Living inside the card means the card's hx-select swap redraws it with the card, the
existing regionWaiting rule (`:popover-open` in the region) holds a live refresh back while it is
open, and decide.js focusMemo already maps a choice in a menu back to the button that opened it.

The dots are drawn outside every `Columns.Show` condition. They are the one control the View menu
cannot take away, which is the whole point.

## Reuse over new routes

Every item posts to a route that exists: transition, priority, star, unstar, the task edit route
(title, version and assignee), tags, tags/remove and delete. None of them is a new service method,
so the capability registry is unchanged. The edit and tag routes used to return only to the task
screen; they now honour a posted `next` exactly as `afterTransition` does for moves, which is the
smallest change that lets a card swap in place. Without `next` they behave as before, so the task
screen's own forms are unaffected.

Restore honours `next` the same way. The toast's Undo form already posts the listing it was shown
on, so undoing a delete from a lane now puts the card back where it was.

A due date is left out: no web form sets one, and adding one means a date parser and a field the
task screen does not have either. That is its own change.

## Rename

Rename reuses the inline title editor from taskopen.js. The title link only carries
`data-edit-title` when the reader chose "Clicking a title edits it", so the menu item carries the
reference and version itself and the editor is started on the card's title link with them. Without
scripting the item is a link to the task screen, whose edit form renames it.

## Comment-needing moves

A move that needs a comment has nowhere to type one in a menu. The status badge leaves those out;
the dots list them as links to the task, so the reader can see the move exists and where to make it.

## Keyboard

Popovers give Escape and focus return for free. Arrow keys are added in lanes.js for any open lane
menu: Down and Up move between its enabled buttons, links and fields, Home and End jump to the ends.
A pure function decides the next index so jstest covers it.
