## ADDED Requirements

### Requirement: Templates in the terminal

The board SHALL open a template picker listing the templates the session can read. Enter SHALL
create a task from the highlighted template in the board's project, and a key SHALL delete the
highlighted template after confirmation. The selected task SHALL be savable as a template under
a typed name.

#### Scenario: Use from the board

- **WHEN** a user opens the picker on the OPS board and presses Enter on "release"
- **THEN** a new OPS task exists and the status bar names it
