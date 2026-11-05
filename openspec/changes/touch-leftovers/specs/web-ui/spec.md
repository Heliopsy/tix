## ADDED Requirements

### Requirement: A toolbar note flows on a narrow or touch-only screen

On a viewport narrower than 900px, or on a device whose primary input cannot hover and is a coarse
pointer, an information disclosure in a toolbar, such as the view switch or the task screen's header,
SHALL open its note in the flow under the toolbar rather than over the content below it. A wider
screen with a hovering pointer MAY keep the note floating.

#### Scenario: Opening the Views note on a phone

- **GIVEN** a viewport 390px wide
- **WHEN** the reader opens the (i) beside the view switch on Tasks
- **THEN** the note is drawn under the toolbar and the listing starts below it

### Requirement: Touch-only devices are not offered drag to move

On a device whose primary input cannot hover and is a coarse pointer, a board card SHALL NOT be
draggable or show a grab cursor, and Settings SHALL NOT draw the Drag to move row. Every card's Move
control SHALL stay available.

#### Scenario: A board on a phone

- **GIVEN** a touch-only phone and Drag to move on
- **WHEN** the reader opens a board
- **THEN** a long press does not lift a card and each card still offers its Move control

### Requirement: The palette offers nothing a touch-only device hides

On a device whose primary input cannot hover and is a coarse pointer, the command palette SHALL NOT
offer the Keyboard section of Settings or the Keyboard shortcuts action, nor any other entry pointing
at something that device does not draw.

#### Scenario: Typing keyboard on a phone

- **GIVEN** a touch-only phone
- **WHEN** the reader opens the palette and types "keyboard"
- **THEN** neither the Keyboard section nor Keyboard shortcuts is offered
