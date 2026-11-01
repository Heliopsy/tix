# Design

## Which route an untick takes

Completion offers every shortest route into a terminal done state, because each done state is a
different outcome. Reopening is different: any open state is "not finished", so the nearest is what
the reader means. The candidates are the routes from the terminal state to non-terminal states with
the fewest hops. In the built-in workflow that is `done → todo`, one route, so the box simply
unticks. A workflow with two equally near open states gets a menu: guessing would write a history
nobody chose, which is the rule `core.FindRoute` already follows.

History was considered ("go back to the state it came from") and rejected for now: it costs one
history read per finished card on every render of every page, and the nearest-state rule gives the
same answer for every workflow that ships.

## One rule for both directions

`tickRoutes` decides both. No transition scope: locked. No candidate: locked. Each candidate is
checked for a step that needs a comment or a scope the reader lacks, and dropped if so. Nothing left:
locked with the first reason. Exactly one candidate in total and usable: a plain tick box. More than
one candidate: a menu of the usable ones, so a choice never becomes a single box just because the
other branch was refused.

## In place

The forms keep `action` and `method`, so without scripting they post and redirect back to the
listing at the card. With htmx each carries `hx-post`, `hx-target` and `hx-select` for its own card
and `hx-indicator` on the card, so the response replaces that card only. The response is the whole
listing page; selecting one card from it means the card is drawn by the same template as every other
card, with no second rendering path.

A refusal is a 4xx page. The shared `htmx:beforeSwap` hook swaps every error page in, which here
would select a card the error page does not have and remove it. `lanes.js` listens on the
document, so it runs after the body's hook, and for a request from a lane card it turns the swap back
off and puts the error's text into the card instead.

## Live updates

`tix.regionBusy` gained two conditions: an element carrying `htmx-request` or a form marked
`aria-busy`, and an open popover. Either defers the refresh, which already re-checks every 500ms.
`applyRegions` remembers which element had focus (the nearest id and its `data-focus` key) and
restores it after the swap, so the refresh triggered by the reader's own change does not drop focus
to the body.

## Phone layout

Below 640px the rest of the task screen already lets the page scroll rather than pinning a frame to
the viewport. The lanes were the exception. Each lane is now its natural height inside a sideways
scroller with mandatory snap, 86vw wide so the next lane peeks in.

## Ordering

This change modifies requirements added by `task-screen-draws-project-lanes`, so it is archived
after that one.

## Two faults found on the way

Both broke lanes and were not lanes' own.

A live refresh writes the listing with `innerHTML`. htmx only wires up markup it swapped in itself,
so every form a refresh drew had no `hx-post` and no boost, and the next tick on any card posted as a
whole page. `applyRegions` and the board's refresh now call `htmx.process` on what they wrote.

The `open` shortcut is bound to Enter, and the keydown handler let only text fields keep their keys.
Enter on a focused button was claimed and did nothing. `tix.pressesNatively` now leaves Enter and
Space on a button or summary, and Enter on a link, to the control.
