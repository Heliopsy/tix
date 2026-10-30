## ADDED Requirements

### Requirement: A star button on every task the browser draws

The task list rows, the project board cards and the task screen SHALL each offer a star toggle for
a reader who may star. The toggle SHALL be a real button inside a form posting with the CSRF
token, SHALL work without JavaScript, SHALL report its state with `aria-pressed`, and SHALL carry
an accessible name saying what pressing it does. Pressing it SHALL star an unstarred task and
unstar a starred one and SHALL return the reader to the screen they were on.

Starred tasks SHALL be drawn first in the list and first within each board column.

#### Scenario: Starring a row

- **GIVEN** a task list with an unstarred task
- **WHEN** the reader presses its star button
- **THEN** the task is starred, the reader is returned to the same listing, and the task is drawn first with its button pressed

#### Scenario: Unstarring from the task screen

- **GIVEN** a starred task's screen
- **WHEN** the reader presses the pressed star button
- **THEN** the task is unstarred and the button is no longer pressed

#### Scenario: No button without the authority

- **GIVEN** a reader who may not star
- **WHEN** the list is drawn
- **THEN** no star button is drawn

#### Scenario: Another open tab follows

- **GIVEN** the same reader with the task list open in a second tab
- **WHEN** they star a task in the first tab
- **THEN** the second tab refreshes from the starred event and draws the task first
