# Task rows keep their shape

## Why

A project imported from a checklist, forty tasks with numbered titles, tags and long Markdown
descriptions, made the task list look broken on a laptop. Below 1700px a row's attributes took
whatever width their content asked for and wrapped where they ran out, so a reference one digit
longer moved every column after it, two long tags squeezed the title to a few letters per line, and
rows ranged from one line to five. The descriptions never reached the row; the list lost its shape
on titles, tags and references alone.

## What Changes

- From 640px up every row of a listing has the same attribute cells at the same widths, so status,
  priority, tags, assignee, project, flags and reference each read down one column. From 1360px the
  cells share the title's line; below that they sit on a line of their own under the title.
- A row's title stops at two lines with an ellipsis, at every width. Tags, the assignee, the project
  and the flags are cut with an ellipsis inside their cell rather than wrapping.
- `scripts/screenshots/shoot.mjs --strict` checks every task row and card: wider than its list or
  column, a Markdown block inside it, more than twice the median row height, or a cell out of line
  with the rows above. `--hostile` seeds tasks with long titles, tags and Markdown descriptions, a
  `laptop` viewport (1440x900) is available, and `just ui-rows` runs the check on the listings.

## Capabilities

### Modified Capabilities

- `web-tasks`: task rows keep aligned columns and a bounded height whatever the content.

## Impact

`internal/web/assets/app.css`, `scripts/screenshots/shoot.mjs`, `justfile`, `docs/testing.md`.
