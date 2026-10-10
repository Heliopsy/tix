# web-tasks Specification

## Purpose

Covers the browser's task screens: the listing, the board, and the controls that switch between them.

## Requirements

### Requirement: The task listing offers a deadline control

The task list screen SHALL offer a control selecting a deadline window from the vocabulary the filter
language's deadline term accepts.

The control SHALL narrow the listing by writing that term into the filter expression the screen parses,
rather than by setting a bound of its own, so that the question asked is visible in the filter box and is
answered by the one shared parser.

The control SHALL show the window currently selected. Its selection SHALL be carried across pages of the
listing. A window the filter language does not accept SHALL be reported beside the filter box, with the
listing withheld, like any other refused expression.

#### Scenario: Selecting overdue narrows the listing

- **GIVEN** one task whose deadline has passed, one whose deadline is far off, and one with no deadline
- **WHEN** the reader selects the overdue window
- **THEN** only the task whose deadline has passed is listed

#### Scenario: The control shows its own state

- **WHEN** the listing is rendered for a selected window
- **THEN** that window is the selected option of the control

#### Scenario: The control and the box ask the same question

- **WHEN** the same deadline term is typed into the filter box instead
- **THEN** the same tasks are listed

#### Scenario: An unknown window is refused, not answered

- **WHEN** a request names a deadline window the filter language does not accept
- **THEN** the message is rendered beside the filter box
- **AND** no rows are listed

### Requirement: A task row marks a deadline only while it is pressing

A row of the task listing SHALL carry a deadline badge when, and only when, the task's deadline classifies
as passed or as near, following the same classification the terminal board follows.

The badge SHALL name the state in words as well as by its colour. It SHALL be one of the listing's
optional columns, so a reader can turn it off.

#### Scenario: An overdue row is badged

- **WHEN** a task's deadline has passed
- **THEN** its row carries a badge naming it overdue

#### Scenario: A distant or absent deadline is not badged

- **WHEN** a task's deadline is further off than the notable window, or the task carries none
- **THEN** its row carries no deadline badge

### Requirement: The task screen names the deadline state

The task screen SHALL name the deadline's state beside the date where the deadline is pressing, in the
same words the listing and the terminal use.

#### Scenario: The task screen says a deadline has passed

- **WHEN** the screen of a task whose deadline has passed is rendered
- **THEN** the deadline is shown with a badge naming it overdue

#### Scenario: A distant deadline is shown without a badge

- **WHEN** the screen of a task whose deadline is further off is rendered
- **THEN** the deadline is shown and no badge is drawn
