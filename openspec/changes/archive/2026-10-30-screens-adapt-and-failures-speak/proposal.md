# A phone gets a strip, a narrow row sheds, and a failure says what failed

## Why

A visual pass over every browser screen changed three behaviours that the specs still describe the
old way. Each was a defect rather than a preference, so the spec has to move rather than the code.

**The board stopped being a board on a phone.** `web-ui` requires its columns to stack one above
another at phone width, and they did. But a board column is a fixed-height scroller so that each
column scrolls its own cards, and a stack of them overran that height: on the project screen the
stacked columns drew straight over the Project settings and Danger zone panels underneath. A
five-state workflow became a 2,800px page with no shape a reader could hold, and finding the one
card you wanted meant scrolling past every state before it. The requirement was written when
"does not scroll sideways" was the whole of the fear, and stacking is the obvious way to honour it,
but what it bought was a page that scrolls five screens vertically instead.

**A task row carried every attribute at every width.** Below 640px the status, priority, tags,
assignee, project, lease, deadline, updated stamp and reference all had to fit beside the title, so
the title was squeezed to a few words. The row is read on a phone to find a task, and the thing that
finds it is the title.

**The error screen never said what went wrong.** The message was rendered with the flash class,
which is a fixed 200px toast pinned to the opposite corner of the page from its own heading. A
reader who hit an error got a heading, two links and no reason, and the one sentence explaining the
failure was either off-screen or read as an unrelated notification.

## What Changes

- **The board is one horizontal strip at every width**, scrolled sideways within its own frame and
  snapped one column to a screen on a phone. The page itself still never scrolls sideways; the
  board does, inside the fixed height that lets each column scroll its own cards. Columns no longer
  wrap onto further rows, which is what escaped that height.
- **A task row sheds its least-identifying attributes as the viewport narrows.** Below 640px the
  tags and the updated stamp come off; below 480px the project chip comes off too. Status,
  priority, assignee, lease, deadline and reference stay at every width. Nothing is lost: the task
  screen lists all of it, one tap away.
- **A failure states itself in flow**, under its own heading and set to a measure, rather than as a
  toast in the corner. The ways out of the screen take the same control the back-link wears.

## Impact

- `internal/web/assets/app.css`: the board strip and its snap, the `.tm-*` row cells and their
  narrow-width rules, the failure block.
- `internal/web/templates/tasks.html`: a cell wrapper per attribute, so a width can address one.
- `internal/web/templates/error.html`: the message in flow, the two ways out as controls.
- `internal/web/layout_test.go`: `TestBoardStaysOneStripOfColumnsAtPhoneWidth` replaces
  `TestBoardStacksItsColumnsAtPhoneWidth`, which asserted the stacking this change removes.
- `openspec/specs/web-ui`: the narrow-width board requirement removed and replaced, two requirements added.

Not changed, and deliberately: the drag-to-move behaviour, which stays off on touch and unchanged
under a pointer; the project board's own settings and danger zone, which the strip stops colliding
with rather than moving; and what the task screen shows, which is where a shed attribute is read.
