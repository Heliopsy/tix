## ADDED Requirements

### Requirement: A card marks a deadline only while it is pressing

A board card SHALL draw a deadline marker when, and only when, the task's deadline classifies as passed or
as near. A card whose deadline is further off, and a card whose task carries no deadline, SHALL draw no
deadline marker.

The marker SHALL be text, so that its meaning survives a terminal receiving no escape sequences, and SHALL
appear in the legend the help overlay renders.

#### Scenario: An overdue card is marked

- **WHEN** a task's deadline has passed
- **THEN** its card's identifier line carries the overdue marker

#### Scenario: A card due within the window is marked differently

- **WHEN** a task's deadline is near
- **THEN** its card carries the near marker and not the overdue marker

#### Scenario: A distant deadline draws nothing

- **WHEN** a task's deadline is further off than the notable window
- **THEN** its card carries no deadline marker, exactly as a task with no deadline does

#### Scenario: The marker survives a colourless terminal

- **WHEN** the board is drawn with colour disabled
- **THEN** the card carries no escape sequence
- **AND** the overdue marker is still present as text

### Requirement: A deadline marker is coloured by the theme

The deadline marker SHALL be rendered through the theme's own style for that deadline state, built the way
the theme's priority and category styles are built, so that it is flattened to the terminal's declared
colour depth and renders as an unstyled string when the theme carries no colour.

The style for a passed deadline, the style for a near one and the dim style the rest of the identifier line
uses SHALL be distinct from one another where colour is available. A deadline that is not notable SHALL
carry no style of its own.

#### Scenario: The marker is not drawn dim

- **WHEN** a card carrying a passed deadline is rendered in colour
- **THEN** the marker is rendered by the theme's overdue style
- **AND** it is not rendered by the dim style the rest of the line uses

#### Scenario: A distant deadline has no style

- **WHEN** the theme is asked for the style of a deadline that is not notable
- **THEN** it returns the theme's plain style

### Requirement: A card carrying a deadline marker still fits its column

A card's identifier line SHALL be fitted to the column's text width measured in terminal cells, marker
included, and SHALL be truncated as one line so that the cut is marked once.

A run of the line cut by that truncation SHALL keep the style it was drawn in.

#### Scenario: The marker does not widen the card past its column

- **WHEN** a card carrying a deadline marker is drawn at any column width down to the board's floor
- **THEN** its identifier line draws no more cells than the column's text width

#### Scenario: A cut marker is still a marker

- **WHEN** the identifier line is truncated inside the deadline marker
- **THEN** the remaining part of the marker keeps the deadline state it was drawn in

### Requirement: The task detail names the deadline state

The task detail SHALL draw the deadline beside the task's other dates and SHALL name its state where the
deadline is pressing, rather than leaving the reader to compare a date with the calendar.

#### Scenario: An overdue task says so

- **WHEN** the detail of a task whose deadline has passed is drawn
- **THEN** the line carries the deadline and names it as overdue

#### Scenario: A distant deadline is stated plainly

- **WHEN** the detail of a task whose deadline is further off is drawn
- **THEN** the line carries the deadline and names no state

#### Scenario: A task with no deadline spends no line on one

- **WHEN** the detail of a task carrying no deadline is drawn
- **THEN** no deadline is drawn at all
