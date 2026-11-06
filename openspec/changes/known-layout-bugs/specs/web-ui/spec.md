## ADDED Requirements

### Requirement: An open info tip stays inside the window

An open info tip's note SHALL lie inside the viewport at every width: a note that would open past an
edge SHALL be moved, and narrowed if the viewport is narrower than it, so the page does not scroll
sideways. Without scripting the note SHALL still open where the stylesheet places it.

#### Scenario: A tip at the right edge of a wide screen

- **GIVEN** a viewport 1920px wide
- **WHEN** the reader opens the tip beside the My work heading
- **THEN** the note is inside the viewport and the page is no wider than the viewport

### Requirement: Long names and tags do not widen a screen

A project name, a user name in an activity line and a tag on a board or lane card SHALL fit their row
or card at every width: a name SHALL wrap, breaking inside a word longer than its line, and a card's
tag SHALL end in an ellipsis with its full name as its title.

#### Scenario: A long project name on a phone

- **GIVEN** a viewport 390px wide and a project named with a 60-letter word
- **WHEN** the projects screen is drawn
- **THEN** the page is no wider than the viewport and the row's name does not run under its links
