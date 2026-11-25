## ADDED Requirements

### Requirement: A project page's header is its name and one line

A project page SHALL draw the project's name with its star and actions, and one line under it holding the
workflow's name as a link to the workflow editor, the task summary and a link to the field definitions.
The summary SHALL NOT count the projects it spans there, and the name SHALL carry no (i).

#### Scenario: The line under the name

- **WHEN** a reader opens `/projects/infra`
- **THEN** one line under the name links the workflow and the field definitions and holds the summary
- **AND** the summary does not say "across 1 project"

### Requirement: Only a shared board names its workflow

The board SHALL draw its line naming the workflow, its columns and the projects sharing it only when it
covers more than one project, with the board's (i) on that same line.

#### Scenario: A one-project board

- **WHEN** a reader opens `/projects/infra?view=board`
- **THEN** no workflow line and no Board (i) is drawn above the columns

#### Scenario: A board of two projects

- **GIVEN** two projects running the same workflow, each with a task
- **WHEN** a reader opens `/tasks?view=board`
- **THEN** the line names the projects sharing the workflow and holds the Board (i)

### Requirement: Quick add and the filter toolbar share a row

On the task screens quick add and the filter toolbar SHALL be drawn in one row, quick add first, and
SHALL sit side by side where the content column is at least 84rem wide. The quick-add preview, the chips
and a refused filter SHALL be drawn after that row.

#### Scenario: A Full HD window

- **WHEN** a reader opens `/projects/infra` in a 1920px wide window with the sidebar open
- **THEN** quick add and the toolbar are on one line, and the tasks start within 200px of the content's top

#### Scenario: A phone

- **WHEN** a reader opens `/projects/infra` on a 390px wide phone
- **THEN** quick add and the toolbar stack, the page does not scroll sideways, and the tasks start within 310px

### Requirement: A board card is two lines with Move in its corner

A board card SHALL draw the star and the title on one line and the reference first on the line of marks
under it. Its Move menu SHALL be a button in the card's corner that a mouse sees on hover or focus and a
touch screen always sees, and SHALL stay a fold holding the plain Move form that drag and drop reads.

#### Scenario: Moving a card from the keyboard

- **GIVEN** a board card with a legal move
- **WHEN** the reader tabs to its Move button and opens it
- **THEN** the button is shown and the Move form posts the transition, with scripting on or off

### Requirement: A project page's quick add has no project picker

Quick add on a project page SHALL add to that project without drawing a project picker. The task list
SHALL keep its picker.

#### Scenario: Adding from a project page

- **WHEN** a reader opens `/projects/infra`
- **THEN** quick add carries the project as a hidden field and draws no project select

### Requirement: The pager over a task listing is one line on a wide screen

On a window 900px or wider the pager over a task listing SHALL be one line holding the position, the
numbered pages and the go-to field, with Previous and Next at its end.

#### Scenario: A listing of four pages

- **GIVEN** a project with four pages of tasks
- **WHEN** a reader opens its page in a 1920px wide window
- **THEN** the pager above the board is one line no taller than 44px
