## RENAMED Requirements

- FROM: `### Requirement: The task screen draws one selection as a list or as a board`
- TO: `### Requirement: The task screen draws one selection as a list, a board or lanes`

## MODIFIED Requirements

### Requirement: The task screen draws one selection as a list, a board or lanes

The task screen SHALL offer a control that switches between a list of rows, a board of columns and
lanes of projects, and SHALL remember the choice per browser so that it survives subsequent requests.

The choice SHALL change only how the selected tasks are drawn. The filter expression, the deadline
window, the sort, the page size, the page position and the project visibility choice SHALL be
resolved once and SHALL apply identically to every view, so that switching never changes which tasks
are on screen beyond the list's open-work default. Every view SHALL be walked by the same pager.

The list SHALL be the view an installation shows before any choice is made, and a stored value that
this build does not recognise SHALL read as the list rather than failing the screen.

#### Scenario: The choice survives the next request

- **GIVEN** a reader on the task screen
- **WHEN** they switch the view to the board or to lanes
- **AND** they request the task screen again
- **THEN** the chosen view is drawn without the switch being pressed a second time

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

## REMOVED Requirements

### Requirement: The task view preference has two states

**Reason**: The preference has three positions now that lanes exist.

**Migration**: Replaced by "The task view preference has no unset position" below, which keeps the
list as the stored-nothing default.

## ADDED Requirements

### Requirement: The task view preference has no unset position

The task view preference SHALL have exactly three positions on every control that offers it: the
list, the board and lanes. No control SHALL offer a position meaning that no choice has been made.

The list SHALL be what an absent, empty or unrecognised stored value means, and choosing the list
SHALL store nothing.

#### Scenario: The switch on settings has the same three positions

- **GIVEN** a reader on the settings screen
- **WHEN** they read the view switch
- **THEN** it offers the list, the board and lanes and nothing else
- **AND** exactly one of them is marked as current

#### Scenario: An untouched browser reads as the list on both screens

- **GIVEN** a browser that has never used either switch
- **WHEN** the reader opens the settings screen and the task screen
- **THEN** both show the list as the current position
