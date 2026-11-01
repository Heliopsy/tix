## ADDED Requirements

### Requirement: Copying in the terminal interface

The terminal interface SHALL bind `Y` to copying the selected task, offered only to a reader who
may copy. It SHALL open a form with the target project, defaulting to the open one, the title
prefilled, and the copy options, and on applying SHALL report the new task's reference.

#### Scenario: Copy from the board

- **WHEN** a reader selects OPS-3, presses `Y`, picks WEB and applies
- **THEN** the status line names the new WEB task

#### Scenario: Not offered without authority

- **WHEN** a reader without task write presses `Y`
- **THEN** no form opens and no copy is made
