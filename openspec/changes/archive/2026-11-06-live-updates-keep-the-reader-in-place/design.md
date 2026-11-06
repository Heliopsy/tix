# Design

## Morph, not replace

The fragment the server renders stays the only rendering. live.js walks the fresh copy beside
the one on screen and patches it: elements are matched by `id` (rows, cards, lanes), then by
`data-group` or `data-state` (board columns, list groups), then by position among unnamed
siblings of the same tag. Attributes and text are synced, kept elements are moved with
`moveBefore` where the browser has it, unmatched old nodes are removed and unmatched new ones
are inserted. A `<details>`'s `open` is the reader's and is never touched. A focused field's
value is never rewritten. idiomorph would do this, but it is another vendored dependency for
about a hundred lines.

The existing busy rule still holds a refresh back while the reader is typing, dragging, waiting
on a request or has a menu open in the region; morphing does not replace it.

## Anchoring

Before patching, every scrolling box around or inside the regions (and the page itself) records
the first named element whose leading edge is in view, per axis. After patching the box is
scrolled by however far that element moved. Chrome's own scroll anchoring may already have done
it; the correction is then zero. New rows (ids starting `t-`) that end above the anchor are
counted into a pill floating over the box's top edge, outside every live region.

## Project control

`project` is folded ahead of the box's expression as `project:KEY` terms, exactly as `due` is,
so one parser answers it on every view. The board already decides its columns from the
projects a filter names: one project, or several that share a workflow, get that workflow's
columns; projects whose workflows disagree get the existing refusal with the list beneath it.
The subscription narrows to the named projects' ids, and widens back to the tenant if a key is
not a current one (an old key still answers as an alias).
