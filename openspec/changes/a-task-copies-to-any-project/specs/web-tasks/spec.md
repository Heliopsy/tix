## ADDED Requirements

### Requirement: Copy to another project in the browser

The task screen and each lane card's more actions menu SHALL offer "Copy to…" to a reader who
may create tasks. It SHALL open a form
with the target project, defaulting to the task's own, the title prefilled and editable, and the
options keep status, with comments, with subtasks, with dependencies and link, link checked. The
form SHALL be drawn in the task dialog when the task is open in one, and as a page otherwise.
After copying, the reader SHALL stay where the form was opened from, the source task or the
listing, with a toast linking the copy, unless
"open after add" is on, in which case the copy opens. The About card SHALL name the task a
copy was copied from.

#### Scenario: Copy and stay

- **WHEN** a reader opens Copy to… on OPS-3, picks WEB and submits
- **THEN** the screen shows OPS-3 again with a toast linking the new WEB task

#### Scenario: Copy and open

- **WHEN** "open after add" is on and a reader copies OPS-3
- **THEN** the copy's task screen opens

#### Scenario: Copied from on the About card

- **WHEN** a reader opens a linked copy of OPS-3
- **THEN** the About card shows "Copied from" with a link to OPS-3

#### Scenario: Copy from a lane card

- **WHEN** a reader opens a lane card's three dots menu on the lanes filtered by `tag:ops`, chooses Copy to… and submits
- **THEN** the reader returns to the lanes filtered by `tag:ops` with a toast linking the copy

#### Scenario: Read-only reader

- **WHEN** a reader without task write opens OPS-3
- **THEN** no Copy to… action is drawn
