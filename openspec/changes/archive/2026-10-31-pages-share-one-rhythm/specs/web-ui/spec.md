## ADDED Requirements

### Requirement: Every screen's heading is spaced the same way

On every signed-in screen, the distance from the page heading block to the first content under it
SHALL be the same at a given viewport width, and a line qualifying the heading SHALL sit the same
distance under the heading on every screen that has one. Both distances SHALL come from shared
tokens, and controls beside the heading SHALL NOT make the heading block deeper.

#### Scenario: A screen with header controls and one without line up

- **GIVEN** a viewport 1440px wide
- **WHEN** the reader opens the API tokens screen, whose header carries a Columns button, and then the
  workflows screen, whose header carries none
- **THEN** the first content under the heading begins the same distance below the heading on both

#### Scenario: A screen opening on a section

- **GIVEN** a screen whose first content under the heading is a section heading
- **WHEN** it is drawn
- **THEN** the section heading begins the same distance under the page heading as a table or a form
  does on other screens

### Requirement: Every screen's content column has the same width cap

On a viewport wide enough to show the sidebar as a column, the content column SHALL be capped by one
measure on every signed-in screen, so its right edge is in the same place on every screen at a given
viewport width. Prose, form fields, forms and charts inside it MAY carry narrower measures of their
own.

#### Scenario: A form screen and a listing end in the same place

- **GIVEN** a viewport 1920px wide
- **WHEN** the reader opens the task screen and then the projects screen
- **THEN** the content of both ends the same distance from the right edge of the window

### Requirement: Controls in one row share one height

Every input, select, button and link styled as a button in the task screen's quick-add row, its
filter row and its header SHALL have the height set by the shared control height token, so no
control in a row is taller or shorter than its neighbours.

#### Scenario: The quick-add row

- **WHEN** the task screen is drawn on a desktop
- **THEN** the title box, the project select, the Add button and the Add with details button are the
  same height

### Requirement: Add with details opens from the quick-add row

Where the fuller add form is offered on the task screen, the control that opens it SHALL be a button
at the end of the quick-add row, at the row's control height, and the form SHALL open over the page
without moving the task list. The form SHALL close on Escape, on a click outside it and on its own
Cancel button, without discarding what was typed. On a phone width the button MAY show its icon
alone and SHALL keep its accessible name.

#### Scenario: Opening the fuller form

- **GIVEN** a reader on the task screen with the advanced screens on
- **WHEN** they press Add with details
- **THEN** the form with title, project, priority, notes, tags and repeat opens under the button
- **AND** the task list stays where it was

#### Scenario: Closing and reopening keeps the draft

- **GIVEN** the reader typed a title into the fuller form
- **WHEN** they press Escape and then Add with details again
- **THEN** the title they typed is still there
