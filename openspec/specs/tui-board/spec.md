# tui-board Specification

## Purpose

Lays out the terminal board: what a card draws, in what order, and what it drops when the column is narrow.

## Requirements

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

### Requirement: A card marks a deadline only while it is pressing

A board card SHALL draw a deadline marker when, and only when, the task's deadline classifies as passed or
as near. A card whose deadline is further off, and a card whose task carries no deadline, SHALL draw no
deadline marker.

The marker SHALL be text, so that its meaning survives a terminal receiving no escape sequences, and SHALL
appear in the legend the help overlay renders.

#### Scenario: An overdue card is marked

- **WHEN** a task's deadline has passed
- **THEN** its card's identifier line carries the overdue marker

#### Scenario: A card due within the window is marked differently

- **WHEN** a task's deadline is near
- **THEN** its card carries the near marker and not the overdue marker

#### Scenario: A distant deadline draws nothing

- **WHEN** a task's deadline is further off than the notable window
- **THEN** its card carries no deadline marker, exactly as a task with no deadline does

#### Scenario: The marker survives a colourless terminal

- **WHEN** the board is drawn with colour disabled
- **THEN** the card carries no escape sequence
- **AND** the overdue marker is still present as text

### Requirement: A deadline marker is coloured by the theme

The deadline marker SHALL be rendered through the theme's own style for that deadline state, built the way
the theme's priority and category styles are built, so that it is flattened to the terminal's declared
colour depth and renders as an unstyled string when the theme carries no colour.

The style for a passed deadline, the style for a near one and the dim style the rest of the identifier line
uses SHALL be distinct from one another where colour is available. A deadline that is not notable SHALL
carry no style of its own.

#### Scenario: The marker is not drawn dim

- **WHEN** a card carrying a passed deadline is rendered in colour
- **THEN** the marker is rendered by the theme's overdue style
- **AND** it is not rendered by the dim style the rest of the line uses

#### Scenario: A distant deadline has no style

- **WHEN** the theme is asked for the style of a deadline that is not notable
- **THEN** it returns the theme's plain style

### Requirement: A card carrying a deadline marker still fits its column

A card's identifier line SHALL be fitted to the column's text width measured in terminal cells, marker
included, and SHALL be truncated as one line so that the cut is marked once.

A run of the line cut by that truncation SHALL keep the style it was drawn in.

#### Scenario: The marker does not widen the card past its column

- **WHEN** a card carrying a deadline marker is drawn at any column width down to the board's floor
- **THEN** its identifier line draws no more cells than the column's text width

#### Scenario: A cut marker is still a marker

- **WHEN** the identifier line is truncated inside the deadline marker
- **THEN** the remaining part of the marker keeps the deadline state it was drawn in

### Requirement: The task detail names the deadline state

The task detail SHALL draw the deadline beside the task's other dates and SHALL name its state where the
deadline is pressing, rather than leaving the reader to compare a date with the calendar.

#### Scenario: An overdue task says so

- **WHEN** the detail of a task whose deadline has passed is drawn
- **THEN** the line carries the deadline and names it as overdue

#### Scenario: A distant deadline is stated plainly

- **WHEN** the detail of a task whose deadline is further off is drawn
- **THEN** the line carries the deadline and names no state

#### Scenario: A task with no deadline spends no line on one

- **WHEN** the detail of a task carrying no deadline is drawn
- **THEN** no deadline is drawn at all
