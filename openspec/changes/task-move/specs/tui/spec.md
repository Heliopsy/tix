## ADDED Requirements

### Requirement: Moving in the terminal interface

The board and the task detail SHALL bind `d` to moving the selected task. It SHALL open a picker of every
project but the task's own, and applying it SHALL call `MoveTask` and name the new reference on the status
bar, with the new status when the old one was not kept.

#### Scenario: Move from the board

- **WHEN** a reader presses `d` on INFRA-A, picks WEB and applies
- **THEN** the task is moved and the status bar reads "moved infra-a to WEB-7"
