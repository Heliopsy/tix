## ADDED Requirements

### Requirement: No screen scrolls sideways

No screen of the web interface SHALL be wider than the window it is drawn in, at any viewport and
whatever the names, titles, tags or descriptions it shows.

#### Scenario: A long name on a phone

- **GIVEN** a user and a project whose names hold a 50 character word
- **WHEN** a screen showing them is drawn at 390px
- **THEN** `document.scrollingElement.scrollWidth` is at most the window's width

### Requirement: Content stays inside its box

Text or an image SHALL NOT pass the right edge of a box that neither clips nor scrolls its overflow
by more than 3px. A box that clips or scrolls has chosen how its overflow is shown.

#### Scenario: An unbroken token in a row

- **GIVEN** a task whose title and tags hold unbroken 60 character tokens
- **WHEN** a listing or board shows it
- **THEN** no text of the task runs past the edge of its row or card

### Requirement: Times are machine-readable and never raw

Every `<time>` element SHALL carry a `datetime` attribute holding a real time, and no visible text or
tooltip SHALL show Go's default time formatting or its zero time.

#### Scenario: A timestamp on a screen

- **WHEN** any screen shows a time
- **THEN** its `<time>` has a `datetime`, and neither its text nor its title holds `0001-01-01`,
  `+0000 UTC` or `m=+`
