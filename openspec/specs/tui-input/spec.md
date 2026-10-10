# tui-input Specification

## Purpose

Governs editing in the terminal: one edit binding dispatched by the view it is pressed in, and the keys that work inside a field.

## Requirements

### Requirement: One key edits the selected thing

The interface SHALL offer exactly one edit binding, dispatched by the view it is pressed in: the
whole-task form on a view where a task is selected, the project form on the project screen, and the
tenant form on the tenant screen. It SHALL NOT offer a second binding that edits a task's title alone,
and no free-text input gathering only a title SHALL remain registered.

Each screen that borrows the binding SHALL describe it in its own words, because the description that
fits a task does not fit a screen holding none.

#### Scenario: The edit key opens the whole-task form

- **WHEN** the edit key is pressed with a task selected, on the board or in the detail view
- **THEN** the whole-task form opens over that task, showing title, body, priority and assignee
- **AND** no single-field title input is opened

#### Scenario: The edit key still edits a project

- **WHEN** the edit key is pressed on the project screen
- **THEN** the project form opens on the open project's attributes

#### Scenario: The edit key still edits a tenant

- **WHEN** the edit key is pressed on the tenant screen
- **THEN** the tenant form opens on the tenant in force

#### Scenario: A reader who may not edit is not offered it

- **WHEN** the reader's authority does not reach the update the binding calls
- **THEN** the footer and the help overlay leave the binding out, and pressing it does nothing

### Requirement: A key cycles the selected task's priority

The interface SHALL offer a binding that moves the selected task one place down the priority scale and
applies it immediately, without opening a picker or any other input. From the lowest priority it SHALL
wrap to the highest, so every priority is reachable from that binding alone.

The binding SHALL be offered wherever a task is selected, gated on the same update operation the rest of
the task actions are gated on, and SHALL NOT be offered on a deleted task.

The binding that opens the priority picker SHALL continue to set any priority in one trip.

#### Scenario: One press moves one place

- **WHEN** the cycle key is pressed on a task at a priority above the lowest
- **THEN** the task is updated to the next priority down, with nothing left open to answer

#### Scenario: The lowest priority wraps

- **WHEN** the cycle key is pressed on a task at the lowest priority
- **THEN** the task is updated to the highest priority

#### Scenario: The cycle reaches the open task too

- **WHEN** the cycle key is pressed in the detail view
- **THEN** the open task's priority moves one place, the same as on the board

#### Scenario: The picker is still one trip

- **WHEN** the picker key is pressed and a priority is chosen
- **THEN** that priority is applied, whatever the task held before

### Requirement: No key means two things in one view

A keybinding scheme SHALL NOT bind one key to two actions reachable from the same view, counting the
cross-view bindings as reachable from every view. A scheme or override that would SHALL be refused
rather than applied.

Because the cross-view bindings are matched before a view's own, a key wanted for a view-level action
SHALL be released by the cross-view action that held it rather than shadowed by it.

#### Scenario: Every shipped scheme validates

- **WHEN** each shipped scheme's key map is validated across every view
- **THEN** no collision is reported

#### Scenario: A key taken by a view action is not also a cross-view key

- **WHEN** a view-level action takes a key a cross-view action held
- **THEN** the cross-view action is bound to a different key, and both keys work

### Requirement: A prose field is edited over several lines

An input gathering prose SHALL render a field several lines tall inside the input panel, seeded with the
value it would replace including its newlines. The field SHALL take at least three lines and at most a
third of the terminal's height, and the panel's key legend SHALL remain the last line on screen.

A task body, a comment and a project description SHALL be gathered as prose. Every other free-text input
SHALL remain one line.

#### Scenario: A body is opened as prose

- **WHEN** the whole-task form is opened on a task whose body runs to several lines
- **THEN** the body field carries every line of the body as written
- **AND** the panel's legend is still the last line of the frame

#### Scenario: A value stays one line

- **WHEN** a tag name, a task reference or a filter expression is gathered
- **THEN** the input is a single line

### Requirement: Enter inserts a newline in a prose field

While a prose field has the cursor, the accept key `enter` SHALL insert a newline rather than applying
the input. A distinct commit key SHALL apply the input from any field, and the panel's own legend SHALL
name both keys while the prose field is open.

#### Scenario: Enter does not submit

- **WHEN** enter is pressed inside a prose field
- **THEN** the input stays open, a newline is inserted, and no call is made

#### Scenario: The commit key applies and is advertised

- **WHEN** a prose field is open
- **THEN** the panel's last line names the key that inserts a newline and the key that applies
- **AND** pressing the commit key applies the input with the text as typed, newlines included

### Requirement: A prose field writes no escape without colour

Where the theme draws no colour, a frame carrying an open prose field SHALL contain no escape sequence.
The field SHALL draw no line numbers and no prompt character of its own.

#### Scenario: A colourless frame with a prose field open

- **WHEN** a prose field is open under a theme drawing no colour
- **THEN** the panel contains no escape sequence and no line numbers

### Requirement: One form edits a task

The interface SHALL offer a form gathering a task's title, body, priority and, where a directory can be
read, its assignee, each showing the value the task currently holds. Cancelling the form SHALL send
nothing. Applying it SHALL send only the fields whose answers differ from the task.

The single-key actions that change one attribute SHALL remain available and unchanged.

#### Scenario: The form states what it will change

- **WHEN** the whole-task form is opened
- **THEN** it draws a row for the title, the body, the priority and the assignee, each carrying the value
  the task holds

#### Scenario: Cancelling writes nothing

- **WHEN** a field is edited and the form is cancelled
- **THEN** no update reaches the service

#### Scenario: Only what changed is sent

- **WHEN** only the body is changed and the form is applied
- **THEN** the update names the body and no other field

### Requirement: A typed field owns the arrows

While a field the reader types into has the cursor, the arrow keys SHALL move the cursor within that
field rather than between fields or between a field's values. Movement between a form's fields SHALL be
bound to keys that no field consumes, and the panel's legend SHALL name them.

#### Scenario: Up moves inside the body

- **WHEN** up is pressed with the cursor in a multi-line body field
- **THEN** the cursor moves within the body and the selected field does not change

#### Scenario: Field movement is always available

- **WHEN** the field-movement key is pressed in any field
- **THEN** the cursor moves to the next field and the legend names that key

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
