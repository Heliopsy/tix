## ADDED Requirements

### Requirement: The task screen draws one selection as a list or as a board

The task screen SHALL offer a control that switches between a list of rows and a board of columns,
and SHALL remember the choice per browser so that it survives subsequent requests.

The choice SHALL change only how the selected tasks are drawn. The filter expression, the deadline
window, the sort, the page size, the page position and the project visibility choice SHALL be
resolved once and SHALL apply identically to both views, so that switching never changes which tasks
are on screen. Both views SHALL be walked by the same pager.

The list SHALL be the view an installation shows before any choice is made, and a stored value that
this build does not recognise SHALL read as the list rather than failing the screen.

#### Scenario: The choice survives the next request

- **GIVEN** a reader on the task screen
- **WHEN** they switch the view to the board
- **AND** they request the task screen again
- **THEN** the board is drawn without the switch being pressed a second time

#### Scenario: Switching keeps the filter

- **GIVEN** a task screen filtered to one tag
- **WHEN** the reader switches to the board
- **THEN** the board holds exactly the tasks the filtered list held
- **AND** the filter expression is still in the filter box

#### Scenario: Switching keeps the project visibility choice

- **GIVEN** a reader who has put one project away
- **WHEN** they switch to the board
- **THEN** no task of the project they put away appears on the board

#### Scenario: An untouched installation shows the list

- **GIVEN** a browser that has never used the switch
- **WHEN** it opens the task screen
- **THEN** the list is drawn

#### Scenario: An unrecognised stored choice shows the list

- **GIVEN** a browser carrying a view preference this build does not implement
- **WHEN** it opens the task screen
- **THEN** the list is drawn rather than an error

### Requirement: Board columns merge only across workflows that are the same state machine

The task board SHALL draw one set of columns from the workflow the selected projects share, and SHALL
treat two projects' workflows as the same workflow only when their definitions agree on every element
a board draws or a move depends on: the states in their declared order, including each state's key,
label, terminal flag and category, and the transitions in their declared order, including each
transition's source, target, required scope and whether it requires a comment.

Agreement SHALL NOT be decided by the workflow's name, key or stored identifier. Two projects whose
workflows are stored separately but agree structurally SHALL merge into one set of columns. Two
projects whose workflows share a name but differ in any compared element SHALL NOT merge.

#### Scenario: Two projects with identical workflows share one board

- **GIVEN** two projects whose workflows are stored separately and describe the same states and transitions
- **WHEN** the reader draws the board over both
- **THEN** one set of columns is drawn, one column per state of the shared workflow
- **AND** the tasks of both projects appear in the column matching their own status

#### Scenario: Two same-named workflows that differ do not merge

- **GIVEN** two projects each running a workflow named `default`, one of which has a state the other does not
- **WHEN** the reader draws the board over both
- **THEN** no merged board is drawn

#### Scenario: A differing state category prevents a merge

- **GIVEN** two projects whose workflows declare the same state keys and transitions but give one state a different category
- **WHEN** the reader draws the board over both
- **THEN** no merged board is drawn

### Requirement: A board card is never offered a move its own project's workflow forbids

Every move a board card offers SHALL be derived from the workflow of that card's own project, never
from the definition the columns were drawn from.

A control for a transition the card's own workflow does not allow SHALL NOT be rendered, so that the
reader is refused before acting rather than by the service after acting. The service refusing such a
move SHALL remain the fallback and SHALL NOT be the only protection.

A drop onto a column SHALL apply a single transition, since a drop names a destination and nothing
else. A route passing through other states SHALL be offered only by a control that names the states
it passes through before it is applied.

#### Scenario: An unreachable state is not offered

- **GIVEN** a card whose project's workflow permits no move from its current state to a given state
- **WHEN** the board is drawn
- **THEN** the card's move control offers no option for that state

#### Scenario: A card offers only its own project's routes

- **GIVEN** a merged board holding cards from two projects
- **WHEN** the board is drawn
- **THEN** each card's move control lists exactly the routes its own project's workflow allows from that card's state

#### Scenario: A multi-hop route names its intermediate states

- **GIVEN** a card whose workflow reaches a state only through another state
- **WHEN** that route is offered
- **THEN** the option names the states passed through and how many steps it takes

### Requirement: A board that cannot be drawn honestly is refused with its reason

When the projects the listing selects do not share one workflow, the task screen SHALL NOT draw a
board of a subset of them. It SHALL draw the list instead, and SHALL state that the board was
refused, naming each distinct workflow and the projects running it.

Each named group SHALL be reachable in one action as a listing narrowed to exactly its own projects,
carrying the reader's filter, deadline window and sort.

The reader's view preference SHALL be left as it is by the refusal, so that narrowing the selection
draws the board without the switch being pressed again.

A task on the page whose status matches none of the drawn columns SHALL be reported on the screen
rather than omitted from it.

#### Scenario: Disagreeing workflows refuse the board and say which projects disagree

- **GIVEN** three selected projects, two sharing a workflow and one differing
- **WHEN** the reader asks for the board
- **THEN** the list is drawn
- **AND** the screen states that the projects do not share a workflow
- **AND** each distinct workflow is named with the projects that run it

#### Scenario: Narrowing to one group draws its board

- **GIVEN** a refused board naming two groups of projects
- **WHEN** the reader follows the entry for one group
- **THEN** the board of that group's shared workflow is drawn
- **AND** the switch did not have to be pressed again

#### Scenario: A task in a state the workflow no longer has is reported

- **GIVEN** a board whose page holds a task whose status is in none of the drawn columns
- **WHEN** the board is drawn
- **THEN** the screen names that task as unplaced rather than leaving it off the board silently

### Requirement: A board of many projects degrades at narrow widths

The task board SHALL remain readable as the number of columns grows with the projects merged into it,
and SHALL NOT make the page scroll sideways. At phone width the columns SHALL be stacked one above
another.

A card on a board covering more than one project SHALL name the project it belongs to.

#### Scenario: Many columns wrap rather than overflow

- **GIVEN** a merged board with more columns than fit the viewport's width
- **WHEN** it is drawn
- **THEN** the columns wrap onto further rows of columns and the page does not scroll horizontally

#### Scenario: Columns stack at phone width

- **GIVEN** a viewport of phone width
- **WHEN** the board is drawn
- **THEN** each column takes the full width, one above another

#### Scenario: A merged card names its project

- **GIVEN** a board merging two projects
- **WHEN** a card is drawn
- **THEN** the card carries its own project's key
