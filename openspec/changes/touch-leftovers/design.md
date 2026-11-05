# Design

## Touch-only is the same media query

Everything hidden here is hidden under `(hover: none) and (pointer: coarse)`, the rule
`settings-read-plainly` introduced, through the same `.needs-*` classes. Drag to move gets its own
class, `.needs-drag`, because what it needs is a pointer that can drag, not hover or keys. The board
card keeps its `draggable` attribute, which the server draws from the setting, and the stylesheet
sets `-webkit-user-drag: none` on it for a touch-only device, so a long press does not lift it. The
script already attached no drag handlers on a coarse pointer.

The palette's entries are JSON the server embeds, so a stylesheet cannot reach them. An entry carries
`keys: true` when it points at something keyboard-only, and `palette.js` drops those when the media
query matches. The server cannot do it: it does not know the device.

## A toolbar note flows below 900px or on touch

A toolbar's (i) floated because opening it should not reflow the toolbar. On a phone the floating
note covered the listing and could run off the edge. Under `(max-width: 899px)` or the touch-only
query the toolbar's tip gives its box away (`display: contents`) and its note becomes a full-row flex
item, as the heading tip does, so it opens under the controls and pushes the listing down. 899px is
the width where the page header already stacks.

## Shooting a touch viewport

Playwright draws a full-page shot under a device metrics override of its own, without the touch
emulation the context set, so during the capture the media query stops matching. The tool now grows
the viewport to the document's height through `setViewportSize`, which keeps the context's touch
and mobile emulation, takes a plain viewport shot, and puts the size back. Desktop viewports keep the
full-page capture.
