## ADDED Requirements

### Requirement: The terminal ticks a checklist item and shows the count

On the board and the detail view, `[` SHALL ask for a checklist item number and flip that item of the
selected task's description, ticking an open item and clearing a ticked one, against the version the
reader is looking at. It SHALL be offered only to a reader allowed `CheckTaskItem`. The detail view
SHALL head the description with how many of its checklist items are done.

#### Scenario: The detail view counts the checklist

- **GIVEN** a task with one of three items ticked
- **WHEN** its detail view is open
- **THEN** the body section is headed `body (checklist 1/3)`
