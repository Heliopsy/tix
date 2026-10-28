## REMOVED Requirements

### Requirement: A board of many projects degrades at narrow widths

**Reason**: Its two narrow-width scenarios required the arrangements that broke the board. A board
column is a fixed-height scroller; both wrapping onto further rows and stacking one above another
grow past that height, and on the project screen the stack drew straight over the Project settings
and Danger zone panels beneath it. Replaced by "The task board is one strip of columns at every
width", which keeps what the requirement was protecting -- a page that never scrolls sideways, and a
card that names its project -- and drops the two arrangements that cost more than they bought.

## ADDED Requirements

### Requirement: The task board is one strip of columns at every width

The task board SHALL remain readable as the number of columns grows with the projects merged into
it, and SHALL NOT make the page scroll sideways.

The board SHALL be drawn as one row of columns at every viewport width, scrolled horizontally within
its own frame. Its columns SHALL NOT wrap onto further rows and SHALL NOT be stacked one above
another, because a board column is a fixed-height scroller and both arrangements grow past that
height, overrunning whatever the page draws beneath the board.

At phone width the board SHALL snap one column to the viewport, so that a column is read whole
rather than in parts.

A card on a board covering more than one project SHALL name the project it belongs to.

#### Scenario: Many columns scroll within the board rather than overflowing the page

- **GIVEN** a merged board with more columns than fit the viewport's width
- **WHEN** it is drawn
- **THEN** the board scrolls sideways within its own frame
- **AND** the page itself does not scroll horizontally

#### Scenario: The board stays one strip at phone width

- **GIVEN** a viewport of phone width
- **WHEN** the board is drawn
- **THEN** the columns are one row scrolled sideways rather than stacked one above another
- **AND** the board's height stays within the viewport rather than running past whatever follows it

#### Scenario: A phone settles on one column at a time

- **GIVEN** a viewport of phone width
- **WHEN** the reader swipes the board sideways
- **THEN** it comes to rest with a single column filling the viewport

#### Scenario: The board does not draw over what follows it

- **GIVEN** a project board with a settings panel beneath it
- **WHEN** the board is drawn at phone width
- **THEN** the settings panel is visible and operable below the board rather than covered by it

#### Scenario: A merged card names its project

- **GIVEN** a board merging two projects
- **WHEN** a card is drawn
- **THEN** the card carries its own project's key

### Requirement: A task row sheds its least-identifying attributes as the viewport narrows

A task row SHALL keep its title legible as the viewport narrows, by omitting the attributes that
identify a task least before the title is compressed.

Below a narrow threshold the row SHALL omit the tags and the last-updated stamp, and below a
narrower one it SHALL also omit the project. The status, the priority, the assignee, the lease
state, the deadline mark and the reference SHALL be shown at every width.

An attribute omitted for width SHALL remain available on the task's own screen, which the row
links to, so that nothing is unreachable rather than merely unshown.

#### Scenario: A phone row keeps its title

- **GIVEN** a task with several tags, an assignee, a project and a deadline
- **WHEN** its row is drawn at phone width
- **THEN** the title is drawn without being compressed to make room for the tags

#### Scenario: The shed attributes are on the task screen

- **GIVEN** a row drawn at a width that omits its tags
- **WHEN** the reader opens the task
- **THEN** the tags are listed there

#### Scenario: Identifying attributes survive every width

- **GIVEN** any viewport width the interface supports
- **WHEN** a task row is drawn
- **THEN** its status, priority, assignee, lease state, deadline mark and reference are present

### Requirement: A failure states itself in the flow of its own screen

When a screen cannot be drawn, the reason SHALL be rendered in the flow of the page, beneath the
heading that announces the failure, and SHALL be set to a measure that keeps it readable as a
sentence.

The reason SHALL NOT be rendered as a transient notification, so that it cannot be placed apart from
the heading it explains, mistaken for an unrelated notice, or drawn outside the reader's view.

The screen SHALL offer the ways out of it as controls carrying the same appearance as the other
navigation controls in the interface.

#### Scenario: The reason sits under its heading

- **GIVEN** a request that cannot be served
- **WHEN** the failure screen is drawn
- **THEN** the reason appears beneath the heading, in the flow of the page

#### Scenario: The reason is not a toast

- **GIVEN** a failure screen
- **WHEN** it is drawn
- **THEN** the reason is not rendered as the transient notification the rest of the interface uses for flashes

#### Scenario: The ways out look like the controls they are

- **GIVEN** a failure screen offering a way back
- **WHEN** it is drawn
- **THEN** that way back is drawn as a control rather than as bare text
