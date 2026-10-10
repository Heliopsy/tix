## ADDED Requirements

### Requirement: Every input mode renders as one panel

A single-line prompt, a numbered picker, a confirmation and a multi-field form SHALL each render as the
same panel: a rule the width of the terminal, the name of what is being asked, one labelled row per
answer, and a final line naming the keys that end it.

A single-line prompt SHALL render as a form with one field, whose label sits in the same column a form's
labels sit in.

#### Scenario: A prompt and a form share a shape

- **WHEN** a single-value prompt is open
- **THEN** the panel names what is being asked, carries one labelled row, and ends with its own legend
- **AND** the label begins in the column a form's labels begin in

#### Scenario: A confirmation states its question and its keys

- **WHEN** a confirmation is open
- **THEN** the panel carries the question naming its subject
- **AND** the line below it names the key that agrees and the key that cancels

### Requirement: An open input mode owns the footer

While any input mode is open, the interface SHALL NOT advertise the bindings of the view behind it. The
only keys named SHALL be the ones that answer the open mode.

#### Scenario: The board's keys are withdrawn under a prompt

- **WHEN** a prompt is opened from the board
- **THEN** the footer no longer names the board's own actions
- **AND** names only the keys that apply or cancel the prompt

### Requirement: An open input mode is visible without colour

The interface SHALL show that input is being captured by means that survive a theme drawing no colour:
the panel SHALL be separated from the body by a rule the width of the terminal, and the panel SHALL carry
its own key legend. Where colour is available the body behind the panel SHALL additionally be dimmed.

#### Scenario: A colourless prompt still shows where focus is

- **WHEN** a prompt is opened with colour disabled
- **THEN** the frame carries no escape sequence
- **AND** a rule the width of the terminal separates the panel from the body

### Requirement: A numbered picker is answered by one keystroke

A numbered picker SHALL continue to be answered by a single digit, and SHALL name the digits it accepts
alongside the key that cancels it.

#### Scenario: One digit performs the transition

- **WHEN** the transition picker is open and its first digit is pressed
- **THEN** the picker closes and the transition is requested
