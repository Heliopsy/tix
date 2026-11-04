# Design

## A note flows instead of floating

The (i) note is a `<details>` whose `<p>` was absolutely positioned under the label. Positioned, it
covers whatever comes next, and on a phone that is the next setting. The lines that hold a note
(`.setname-line`, `.field-label`, `.cfield-label`, `.headtip`) become block boxes whose children are
inline, and the `<details>` itself is inline. A block inside an inline box is laid out as a full
width block after the line, which is exactly "open under the name and push the rest down", with no
script and in every engine. A flex line cannot do it: a flex item is blockified, so the note would
widen the `<details>` beside the label instead of dropping under it.

Notes in toolbars (the view switch, a page's header actions) keep floating, since there is no row
below them to push and the toolbar should not reflow when one opens.

## Two named positions are a segmented control

A switch says on or off. Page and Dialog are both "on"; the old switch drew Page with the track off,
which reads as "the page is off". The segmented control the Task view and List groups rows already
use names both positions and marks the current one with `aria-current`. Each segment is a submit
button carrying the position, so it works with scripting off. The handler sets the named position;
with none named it flips as before, so a cached form or a script posting the old shape is answered
the same way.

## Touch-only means the main input is a finger

`(hover: none) and (pointer: coarse)` describes the primary input. A phone or a tablet matches; a touch
laptop, whose primary input is its trackpad, does not and keeps the rows. `any-hover` would also keep
them on a tablet with a keyboard attached, but no engine's device emulation reports it, so the rule
could not be checked in a screenshot, and a phone is the case that matters.
