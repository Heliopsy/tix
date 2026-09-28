## ADDED Requirements

### Requirement: A card leads with its title

A board card SHALL draw the task's title first, across the column's text width, and SHALL draw the
reference, the priority and the task's markers beneath it in the dim style.

A title that does not fit on one line SHALL wrap onto at most one further line, broken on a word where
the line allows it, and the continuation SHALL begin in the same text column as the first line. Only the
last line drawn SHALL be truncated, and truncation SHALL be marked with an ellipsis.

A card whose title fits on one line SHALL occupy one line of title and no more.

#### Scenario: A title that fits takes one line

- **WHEN** a task's title fits the column's text width
- **THEN** the card draws one title line and one line of identifiers

#### Scenario: A long title wraps to an indented continuation

- **WHEN** a task's title is longer than the column's text width
- **THEN** the title is broken on a word and continued on a second line
- **AND** the second line starts in the same column as the first
- **AND** only the second line may carry an ellipsis

#### Scenario: A title too long for two lines is cut once

- **WHEN** a task's title needs more than two lines
- **THEN** the second line carries what remains, truncated with an ellipsis
- **AND** no third line is drawn

### Requirement: A card names its priority apart from its markers

A card's identifier line SHALL separate the reference, the priority and the marker group from one
another, so that a priority and a marker are never rendered as one token.

The priority SHALL render as `P` followed by the priority's digit, one through five.

#### Scenario: A task with a due date

- **WHEN** a task of high priority carries a due date
- **THEN** the card reads the reference, then the priority, then the due-date marker, separated

#### Scenario: A task with no markers

- **WHEN** a task carries no markers
- **THEN** the card's identifier line carries the reference and the priority only

### Requirement: Selection is a shape, not a colour

The board SHALL draw a bar down the left edge of every card, and the bar of the selected card SHALL use a
different glyph from the bar of an unselected card. The two glyphs SHALL occupy the same number of cells.

A frame rendered by a theme that draws no colour SHALL carry no escape sequence and SHALL still
distinguish the selected card.

#### Scenario: A colourless board still says what is selected

- **WHEN** the board is rendered with colour disabled
- **THEN** the frame carries no escape sequence
- **AND** the selected card's bar differs from every unselected card's bar

#### Scenario: The selected bar is on the selected card

- **WHEN** a card is selected
- **THEN** the selected bar is drawn beside that card's own lines and no other's

### Requirement: Column width follows what a column has to show

The board SHALL share the width available to the columns on screen in proportion to what each column has
to show, rather than equally.

A column holding tasks SHALL be at least the board's minimum column width. A column holding no tasks MAY
be narrower, but SHALL be wide enough for its own heading. The columns on screen SHALL together occupy the
whole width they were given.

#### Scenario: An empty column does not take a busy column's room

- **WHEN** the board holds a column of eight tasks and a column of none
- **THEN** the empty column is drawn narrower than the busy one
- **AND** the busy column is at least the minimum column width

#### Scenario: The board uses all of its width

- **WHEN** the columns on screen are laid out
- **THEN** their widths and the gaps between them total the terminal's width

### Requirement: A column is as tall as what it holds

A column SHALL be drawn to the height of its own contents, up to the height of the body. A column holding
no tasks SHALL NOT be drawn to the full height of the body.

#### Scenario: An empty column is a short box

- **WHEN** a column holds no tasks
- **THEN** its box is shorter than the body
- **AND** shorter than the box of a column holding tasks

### Requirement: A column's window keeps the selected card whole

A column that cannot draw every card SHALL choose its window by the lines its cards occupy, SHALL keep
the selected card wholly on screen, and SHALL state how many cards are hidden above and below.

#### Scenario: Scrolling past the bottom of a column

- **WHEN** the selection moves to a card below the column's last visible one
- **THEN** the window moves so that card is drawn in full
- **AND** the column states how many cards are hidden above and below it
