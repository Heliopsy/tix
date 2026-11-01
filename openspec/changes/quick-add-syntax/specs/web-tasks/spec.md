## ADDED Requirements

### Requirement: The one-line add box reads quick-add syntax

The task listing's one-line add box SHALL read its line through the quick-add parser, using
the reader's chosen time zone, with a project the line names taking the place of the box's
project picker. Its (i) tip SHALL list the syntax. While the reader types, a row of chips
under the box SHALL show what the line will set, and a line that cannot be parsed, or that
names a project or handle the tenant does not have, SHALL show the refusal there. A refused
add SHALL redraw the listing with the line still in the box and the refusal under it. Adding
SHALL work with scripting off.

#### Scenario: Preview

- **WHEN** the reader types `Fix login !high #auth` into the box
- **THEN** chips for priority high and tag `auth` appear under it

#### Scenario: Refusal keeps the line

- **WHEN** the reader submits `Fix login @nobody` and no actor has that handle
- **THEN** the listing is drawn again with `Fix login @nobody` in the box and a message naming the handle under it, and no task is created
