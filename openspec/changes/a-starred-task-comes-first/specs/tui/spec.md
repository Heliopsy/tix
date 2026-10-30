## ADDED Requirements

### Requirement: The terminal interface stars a task

The terminal interface SHALL toggle the star on the selected task with the `*` key, offered only
to a reader who may star and unstar, and listed in the help overlay and the command palette. A
starred task SHALL be marked on its card and SHALL be drawn first within its board column.

#### Scenario: Toggling a star

- **GIVEN** an unstarred task selected on the board
- **WHEN** the reader presses `*`
- **THEN** the task is starred, the board reloads, and the card is marked and first in its column

#### Scenario: The key is offered only with the authority

- **GIVEN** a reader who may not star
- **WHEN** the help overlay is opened
- **THEN** the star key is not listed and pressing it calls nothing
