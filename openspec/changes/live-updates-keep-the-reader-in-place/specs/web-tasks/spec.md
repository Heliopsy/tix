## ADDED Requirements

### Requirement: A live refresh keeps the elements still on screen

A live refresh SHALL patch the regions it redraws in place. Every row, card, board column, lane
and scrolling box present before and after SHALL remain the same element, only rows that are new
SHALL be inserted, and a disclosure the reader opened SHALL stay open.

#### Scenario: A burst while the reader scrolls

- **GIVEN** the task list, board or lanes scrolled down and sideways
- **WHEN** a burst of changes refreshes the screen
- **THEN** the scrolling boxes, the rows and the cards on screen are the same elements as before and the reader's focus stays on the element it was on

### Requirement: A live refresh holds the reader to what they were reading

After a live refresh each scrolled box SHALL show the row and the column the reader had at its top
and left edge before it, on both axes. Rows inserted above that row SHALL NOT move it; a pill
SHALL say how many arrived and scroll to them when pressed.

#### Scenario: New tasks above a scrolled list

- **GIVEN** the task list scrolled down
- **WHEN** three tasks that sort above the rows in view are created
- **THEN** the row at the top of the list does not move and a pill reads "3 new, show"

### Requirement: A burst redraws a listing at most twice a second

A listing's live refresh SHALL coalesce events over 500ms, drawing the first at once and the last
of a burst after it. A background refresh SHALL NOT show a busy pointer or mark anything busy, and
SHALL NOT add or remove a scrollbar on a box whose content did not change size.

#### Scenario: No flicker during a burst

- **WHEN** a burst of changes refreshes the task list
- **THEN** no element shows a progress pointer and no scrolling box gains or loses a scrollbar

### Requirement: The task screen filters by project in every view

The filter bar SHALL offer a project control on the list, the board and the lanes. It SHALL
submit `project` with one key or several comma separated, read as `project:` terms of the filter.
The view switch, the pager, the live refresh, CSV export and a saved filter SHALL keep it.

#### Scenario: A board of one project

- **WHEN** the reader picks a project whose workflow has its own states and switches to the board
- **THEN** only that project's cards are shown, in that workflow's columns, and the control still names the project

#### Scenario: Projects that disagree

- **WHEN** the reader asks for two projects whose workflows differ and the board is chosen
- **THEN** the screen explains that they cannot share a board and shows the list

### Requirement: A project-filtered screen listens only to that project

A task screen filtered to projects by their current keys SHALL subscribe to events of those
projects only, so a change elsewhere neither wakes nor appears on it.

#### Scenario: A change in another project

- **GIVEN** the board filtered to project web
- **WHEN** a task is created in project infra
- **THEN** the board's refresh does not draw it
