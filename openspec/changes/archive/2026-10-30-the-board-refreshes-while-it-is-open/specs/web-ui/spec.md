## ADDED Requirements

### Requirement: A live region is held open only by work the reader did

A screen that defers its refresh while the reader is mid-edit SHALL decide
that from what the reader changed, and not from what the browser did on its
own.

A `<select>` whose rendered markup names no option as selected is presented by
every browser with its first option selected. That presentation SHALL NOT be
read as an edit: such a control SHALL be treated as untouched while it sits on
its first option, and as edited once it sits on any other. A `<select>` whose
markup does declare a default SHALL continue to be read against that default.

A control rendered inside a live region SHALL declare its own default, so that
what the reader is shown and what the markup asked for agree. The project
board's Move control SHALL mark its first route selected.

A live region SHALL NOT be reported as holding work in progress on a page that
has only been rendered.

#### Scenario: A board that has only been rendered updates

- **GIVEN** an open project board whose cards each offer a Move control
- **WHEN** a task on it changes anywhere else and the event reaches the screen
- **THEN** the board re-fetches its fragment and shows the change, rather than deferring

#### Scenario: A move the reader picked is not thrown away

- **GIVEN** a reader who has opened a card's Move menu and chosen a column
- **WHEN** an event arrives for that board
- **THEN** the refresh is deferred until the menu is back at its default, and then lands

#### Scenario: The Move control names the option the browser will show

- **WHEN** the board is rendered, as a whole page or as its live fragment
- **THEN** each Move select carries exactly one selected option, and it is the first route

#### Scenario: A declared default is still what a choice is measured against

- **GIVEN** a select whose markup marks an option other than the first as selected
- **WHEN** the control sits on the first option instead
- **THEN** it is reported as edited
