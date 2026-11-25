## ADDED Requirements

### Requirement: A task's activity is tabs with counts

The task screen and the task dialog SHALL show Comments, History and Artifacts as a tab strip under the
task's dependencies, each tab naming its count and a zero count shown muted. One panel SHALL show at a
time, the tabs SHALL follow the ARIA tabs pattern with arrow keys, Home and End, and none SHALL sit in
an expander.

#### Scenario: Switching with the keyboard

- **GIVEN** a task screen showing the Comments tab
- **WHEN** the reader focuses it and presses the right arrow
- **THEN** the History tab is selected and focused and only its panel is shown

### Requirement: The activity tab chosen is kept and addressable

The browser SHALL remember the activity tab a reader last chose. A fragment naming a panel or a record
inside one, an older page of history, or a raw entry in the address SHALL open the tab that holds it
instead. A live update SHALL redraw the lists and counts and keep the tab shown.

#### Scenario: A history link opens History

- **GIVEN** a reader whose last chosen tab was Artifacts
- **WHEN** they follow a link to the task ending in `#history`
- **THEN** the History tab is the one shown

#### Scenario: A comment arrives while History is shown

- **WHEN** a live update brings a new comment
- **THEN** the Comments count goes up and History stays the tab shown

### Requirement: The activity panels all show without scripting

Without scripting the tab strip SHALL NOT be drawn and every activity panel SHALL show under its own
heading, one under another.

#### Scenario: Scripting off

- **WHEN** a task screen is drawn with scripting disabled
- **THEN** Comments, History and Artifacts each show with their heading and nothing is hidden

### Requirement: The task panel's due hint fits its column

The due field in the task panel SHALL show a hint that fits its column whole: the date in the reader's
format, with the words and offsets it also takes in its tip.

#### Scenario: A task with no due date

- **WHEN** a task with no due date is shown at any viewport
- **THEN** its due field's hint is a whole date, not one cut off

## MODIFIED Requirements

### Requirement: An activity row's subject leads to the record itself or names its kind

An activity row's subject SHALL link to the record itself where the reader may reach one: a task to
its own screen, a project or workflow to its screen, and a comment or an artifact to its place on
the task it hangs off, addressed so the browser lands on that record rather than on the top of the
page. A subject the feed cannot resolve SHALL name its kind in words and carry no link, never a bare
identifier presented as a destination.

#### Scenario: A comment is reachable from the row about it

- **WHEN** the feed renders an entry whose subject is a comment
- **THEN** the row links to that comment's own place on the task it was left on

#### Scenario: A record inside a closed disclosure is still reached

- **WHEN** a link from the feed addresses a record that the target screen keeps behind a disclosure
  or on a tab not shown
- **THEN** that disclosure is opened or that tab shown, and the record is marked, so the link lands
  somewhere visible

#### Scenario: An unresolvable subject is not a dead link

- **WHEN** the feed renders an entry whose subject it cannot name
- **THEN** the row says what kind of record changed and offers no link
