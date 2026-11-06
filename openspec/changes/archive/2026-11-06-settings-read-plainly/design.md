# Design

## A note flows instead of floating

The (i) note is a `<details>` whose `<p>` was absolutely positioned under the label. Positioned, it
covers whatever comes next, and on a phone that is the next setting. Two layouts replace it, with no
script either way.

A setting's name and a field's label (`.setname-line`, `.field-label`) become block boxes whose
children are inline, and the `<details>` itself is inline. A block inside an inline box is laid out as
a full width block after the line, which is exactly "open under the name and push the rest down".

A heading or a create form's label (`.headtip`, `.cfield-label`) stays a flex line, because its large
heading or its right-aligned "required" mark lines up badly as inline content. The line wraps, and the
`<details>` is `display: contents`, so its note is a flex item of its own with a full-row basis. An
engine that does not honour `contents` on `<details>` draws the note beside the heading instead, which
is no worse than before.

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
