## ADDED Requirements

### Requirement: The web binds the shared key table

The web's keyboard schemes SHALL bind each action of the key table it shares with the app that the web
can perform, with the keys the table gives it, and SHALL bind no key to an action the web cannot
perform. Go to Manage and Delete SHALL stay unbound on the web.

#### Scenario: The same key on both surfaces

- **GIVEN** the default scheme
- **WHEN** the reader presses `a` with a task screen open
- **THEN** focus moves to the task's assignee field, as `a` assigns in the app

#### Scenario: Vim's a assigns

- **GIVEN** the vim scheme
- **WHEN** the reader presses `a` on a task screen
- **THEN** the assignee field takes focus and the comment field does not

### Requirement: Two keys in a row reach a screen

Pressing `g` and then a second key within one second SHALL open the screen the sequence names: `i`
Inbox, `m` My work, `t` Tasks, `p` Projects, `a` Activity, `s` Settings and `S` Statistics. In the
default scheme a `g` followed by no second key SHALL jump to the first task when the second has passed.

#### Scenario: Go to the inbox

- **WHEN** the reader presses `g` and then `i`
- **THEN** the inbox opens

#### Scenario: A lone g

- **GIVEN** the default scheme and a task list
- **WHEN** the reader presses `g` and nothing else for a second
- **THEN** the first task is focused

### Requirement: A waiting key gives way

A second key that completes no sequence SHALL drop the wait and act as it would alone, Escape SHALL
only drop the wait, and typing into a field SHALL drop it without running anything.

#### Scenario: g then j

- **WHEN** the reader presses `g` and then `j` on a task list
- **THEN** the selection moves down and does not jump to the top afterwards

### Requirement: Task keys press the screen's own controls

The star, priority, assign, due, snooze, tag and edit description keys SHALL press or focus the
control the screen draws for that action, on the focused row when a row is focused, and otherwise a
control outside every row. Where there is no such control the key SHALL be left to the browser.

#### Scenario: Star the focused row

- **GIVEN** a task list with the second row focused
- **WHEN** the reader presses `s`
- **THEN** that row's star is pressed

#### Scenario: Nothing focused on a list

- **GIVEN** a task list with no row focused
- **WHEN** the reader presses `s`
- **THEN** no task is starred and the key is not claimed

### Requirement: Copy reference, refresh and switch view have keys

`y` SHALL copy the focused row's reference, or the open task's, and say it was copied. `r` SHALL
redraw the screen's live regions, or reload a screen without them. `v` (`g v` in Vim) SHALL press
the next of the list, board and lanes views where the screen draws that switch.

#### Scenario: Copy a reference

- **GIVEN** the task INFRA-5 is open
- **WHEN** the reader presses `y`
- **THEN** INFRA-5 is on the clipboard

#### Scenario: Switch from list to board

- **GIVEN** the task list drawn as a list
- **WHEN** the reader presses `v`
- **THEN** the tasks are drawn as a board

### Requirement: An Off scheme binds chords only

The schemes SHALL include Off, stored as `off` in the scheme cookie and offered on the settings
screen, which binds no single key and no sequence, keeping only ctrl+k and meta+k for the palette and
ctrl+a and meta+a for select all.

#### Scenario: Typing a letter with Off

- **GIVEN** the Off scheme
- **WHEN** the reader presses `j`, `g` or `?` outside a field
- **THEN** nothing happens

#### Scenario: The palette with Off

- **GIVEN** the Off scheme
- **WHEN** the reader presses ctrl+k
- **THEN** the command palette opens

### Requirement: Emacs keeps select all on ctrl+a

The emacs scheme SHALL jump to the first and last task with alt+< and alt+> as well as Home and End,
and SHALL leave ctrl+a as select all. It SHALL also open the palette with alt+x, page with ctrl+v and
alt+v and select with ctrl+space.

#### Scenario: ctrl+a in Emacs

- **GIVEN** the emacs scheme and a task list in select mode
- **WHEN** the reader presses ctrl+a
- **THEN** every task in the list is selected

### Requirement: The help overlay groups the keys

The help overlay SHALL list the active scheme's keys under the headings Navigate, Lists, Task and
General, in that order, with the action labels the app uses, leaving out an action the scheme binds
no key to and a heading with nothing under it.

#### Scenario: Help under Off

- **GIVEN** the Off scheme
- **WHEN** the reader opens the help from the account menu
- **THEN** it shows Lists with select all and General with the palette, and no Navigate or Task heading
