## ADDED Requirements

### Requirement: Saved filters in the terminal

The board SHALL offer a picker listing the reader's saved filters and the shared ones; choosing one SHALL
apply its expression and sort to the filter bar, and Escape SHALL close only the picker. The board SHALL
offer saving the active filter under a name. Both SHALL be in the command palette and offered only to a
reader with `task:read`.

#### Scenario: Apply a saved filter

- **GIVEN** a saved filter `ops` holding `tag:ops`
- **WHEN** the reader picks it
- **THEN** the board shows only the `tag:ops` tasks and the filter bar reads its expression
