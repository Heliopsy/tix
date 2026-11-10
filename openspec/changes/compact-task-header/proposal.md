# A compact header on the task screens

## Why

After the filter toolbar was compacted, the top of a project page was still tall. On a Full HD window
the board started about 290px down: a title row, a workflow line, a summary line, quick add, the
toolbar, the board's own workflow line repeating the first, and an (i) stranded on a line of its own.
Every board card spent a line on its reference and another on a Move disclosure.

## What Changes

- A project page's header is the name, its star and its actions, with one muted line under the name:
  the workflow (a link to its editor), the summary and the field definitions. The summary no longer
  says "across 1 project" there, and the (i) beside the name is gone; the (?) help says the same.
- The board draws its workflow line only when it is several projects' board, with its (i) on that
  line. A one-project board says nothing the header has not.
- Quick add and the filter toolbar share one row where the content column is at least 84rem wide,
  quick add on the left with Add with details and From template as icon buttons. Narrower, they
  stack. The quick-add preview, the chips and a refused filter are drawn after the row.
- A board card is two lines: the star and the title, then a small line led by the reference. Move is
  a button in the card's corner, shown on hover or focus with a mouse and always on a touch screen.
  It stays a fold holding a plain form, so it and drag and drop work as before.
- The screenshot check fails tasks that start more than 150px below the top of the content on a Full
  HD window, or 240px on a phone, when nothing in force is meant to push them down.

## Capabilities

### Modified Capabilities

- `web-tasks`: the task screens' header, the row quick add shares with the toolbar, the board's
  workflow line and the board card.

## Impact

`internal/web/templates/tasks.html`, a new stylesheet part `740-compact-task-header.css`, two
Bulgarian strings removed with the (i) they translated, `docs/web-ui.md`, `scripts/screenshots/shoot.mjs`.
