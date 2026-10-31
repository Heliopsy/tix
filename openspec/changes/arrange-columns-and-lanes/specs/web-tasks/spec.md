## ADDED Requirements

### Requirement: Every group heading carries a menu

Each column of the task screen's board, each lane of the lanes view, each column of a project's
board and each group heading of a grouped list SHALL carry a three dots button opening a menu of
actions on that group. The button SHALL be visible while the pointer is over the heading or focus
is inside it, and SHALL always be visible on a coarse pointer, where it and every menu item SHALL be
at least 40px tall. Escape SHALL close only the menu and return focus to its button; keyboard
shortcuts SHALL not act and a live refresh SHALL wait while the menu is open. Every action SHALL be
a form post that works without scripting and, with scripting, SHALL redraw the view in place
without reloading the page.

#### Scenario: Hovering a column reveals its dots

- **GIVEN** the task screen drawn as a board
- **WHEN** the reader moves the pointer over the `todo` column heading
- **THEN** its three dots button becomes visible and opens a menu of actions on `todo`

#### Scenario: The menu works without scripting

- **WHEN** the reader submits Hide from a column's menu in a browser without JavaScript
- **THEN** the screen returns to the same listing with the column hidden

### Requirement: A reader hides, stars, collapses and reorders groups

The menu SHALL offer to hide the group, to star or unstar it, to collapse or expand it, and to move
it left or right (up or down in a list). A hidden group SHALL not be drawn. A starred group SHALL be
drawn before every unstarred one. A collapsed group SHALL be drawn as a strip naming the group and
its count with no cards or rows. Move SHALL swap the group with its drawn neighbour of the same star
state and SHALL be absent where there is none. The arrangement SHALL be stored per actor and per
view on the server, so it applies in any browser the reader signs in to, and SHALL never change
what another reader sees.

#### Scenario: A hidden column stays hidden in another browser

- **GIVEN** a reader who hid the `done` column of the task screen's board
- **WHEN** they sign in from a fresh browser and open the board
- **THEN** the `done` column is not drawn

#### Scenario: A starred lane comes first

- **GIVEN** lanes for projects `alpha` and `beta`, in that order
- **WHEN** the reader stars the `beta` lane
- **THEN** the `beta` lane is drawn before `alpha`

#### Scenario: Another reader is unaffected

- **WHEN** one reader hides a column
- **THEN** another reader of the same tenant still sees it

#### Scenario: A collapsed column still takes a dropped card

- **GIVEN** a collapsed `doing` column
- **WHEN** the reader drags a card from `todo` onto it
- **THEN** the task moves to `doing`

### Requirement: Hidden groups are counted and can be shown again

When a view hides any group it would otherwise draw, a control at the end of the view SHALL say how
many are hidden and offer to show each one, and the task screen's View menu SHALL list the same
groups with the same choice and offer to reset the arrangement of the view in force.

#### Scenario: Showing a hidden column again

- **GIVEN** a reader who hid the `done` column
- **WHEN** they press Show beside `done` under "Hidden: 1"
- **THEN** the `done` column is drawn again

### Requirement: A group's menu offers what belongs to its kind

A project group's menu SHALL link to the project, to the task screen filtered to the project, and
SHALL offer a field adding a task to that project. A status group's menu SHALL link to the task
screen filtered to that status and SHALL offer a field adding a task in that state, to a project the
group draws, when the reader may create tasks.

#### Scenario: Adding a task from a column

- **WHEN** the reader adds "Write release notes" from the `doing` column's menu
- **THEN** a task with that title is created in state `doing` and the reader stays on the board

### Requirement: The list can group its rows

The list view SHALL offer a choice, kept per browser, to draw its rows ungrouped, grouped by project
or grouped by status. A grouped list SHALL draw one heading per group present on the page, carrying
the group's name, its count and the group menu, and SHALL share the arrangement of the lanes view
when grouped by project and of the board when grouped by status.

#### Scenario: Grouping the list by project

- **WHEN** the reader chooses "Group by project" under View
- **THEN** the list draws a heading per project with its rows under it

#### Scenario: A lane hidden in the lanes view is hidden in the grouped list

- **GIVEN** a reader who hid the `homelab` lane
- **WHEN** they draw the list grouped by project
- **THEN** no `homelab` group is drawn and the hidden count names it
