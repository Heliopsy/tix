## ADDED Requirements

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
