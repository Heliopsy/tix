## ADDED Requirements

### Requirement: Copy an activity event as JSON

On the activity view `^` SHALL copy the selected event as pretty-printed JSON to the terminal's clipboard with OSC 52 and draw what it copied.

#### Scenario: Copying the selected event

- **WHEN** a reader selects an event on the activity view and presses `^`
- **THEN** the clipboard request carries that event as indented JSON
- **AND** the copied text is shown on screen
