## ADDED Requirements

### Requirement: A command palette finds tasks, projects, screens and actions by name

Every signed-in screen SHALL carry a command palette, a modal dialog with one search box over a list
grouped into Tasks, Projects, Screens and Actions. It SHALL open on ctrl+k and on command+k in every
shipped key scheme, including while a text field has focus, and from a Search button in the sidebar
and in the phone's bar. The "?" overlay SHALL list its key. Screens SHALL be every sidebar destination
the reader's sidebar shows and every section of Settings. Actions SHALL include New task, New task in
a named project, New project, Open the filter, Show tasks as a list, a board and in lanes, Switch the
colour scheme, Keyboard shortcuts and Sign out, each driving a control the interface already has.
Every label SHALL be in the page's language. The sign-in screen SHALL NOT carry the palette.

#### Scenario: Opening from the keyboard

- **GIVEN** a signed-in reader on any screen, in any key scheme
- **WHEN** they press ctrl+k, or command+k on a Mac
- **THEN** the palette opens with the search box focused
- **AND** the screens and actions are listed at once

#### Scenario: The overlay names the key

- **WHEN** the reader presses "?"
- **THEN** the overlay lists ctrl+k and meta+k against Open the command palette

#### Scenario: A Bulgarian reader

- **GIVEN** a reader whose language is Bulgarian
- **WHEN** they open the palette
- **THEN** its placeholder, group names, screens and actions are in Bulgarian

### Requirement: The palette asks the server for tasks only once something is typed

Screens and actions SHALL be embedded in the page and matched in the browser with a fuzzy match.
Once something is typed, the group holding the best match SHALL be listed first, and an answer
arriving from the server SHALL NOT move the selection off the option the reader had selected.
Projects SHALL be read from `GET /palette/projects` when the palette opens. Tasks SHALL be asked for
from `GET /palette/tasks?q=` only when the search box holds text, once the reader has paused typing
for 150ms, and an answer to an older query SHALL NOT be drawn. The task search SHALL read the query
with the filter language, falling back to plain words when it does not parse, SHALL put the task a
ref-shaped query names first, SHALL leave out deleted tasks, and SHALL answer at most ten tasks with
keyset pagination. Both routes SHALL require a signed-in reader and SHALL answer only the session
tenant's records. They SHALL add no capability: the API answers the same through
`GET /api/v1/tasks?q=`, `GET /api/v1/tasks/{ref}` and `GET /api/v1/projects`.

#### Scenario: Opening sends no task search

- **WHEN** the reader opens the palette
- **THEN** no request is made for tasks

#### Scenario: Typing searches once

- **WHEN** the reader types "dep" quickly, one letter at a time
- **THEN** one task search for "dep" is made after they pause
- **AND** the matching tasks are listed under Tasks

#### Scenario: A ref finds its task

- **WHEN** the reader types a task's ref in any case
- **THEN** that task is the first under Tasks

#### Scenario: Another tenant's task is never offered

- **GIVEN** a task in another tenant whose title matches the query
- **WHEN** the reader searches for it by title or by ref
- **THEN** the palette offers nothing for it

#### Scenario: A signed-out request is refused

- **WHEN** a browser without a session asks either palette route
- **THEN** it is not answered with JSON

### Requirement: The palette is driven from the keyboard and returns focus

ArrowDown and ArrowUp SHALL move the selection through every option, wrapping at either end, Enter
SHALL choose the selected option, a click SHALL choose the option clicked, and Escape, the Close button
or a click on the backdrop SHALL close the palette. Tab SHALL stay inside the palette. When the
palette closes without leaving the page, focus SHALL return to the element that had it. The search
box SHALL be a combobox naming the selected option with `aria-activedescendant`. On a coarse pointer
each option SHALL be at least 44px tall, and under 600px the palette SHALL fill the screen.

#### Scenario: Choosing with the arrows

- **WHEN** the reader presses ArrowUp on the first option and then Enter
- **THEN** the last option is chosen

#### Scenario: Escape puts focus back

- **GIVEN** the reader opened the palette from a task row
- **WHEN** they press Escape
- **THEN** the palette closes and the row has focus again

### Requirement: A task chosen in the palette opens the way the reader chose

Choosing a task SHALL open it in the task dialog when Task opens in is Dialog and the page carries
the dialog, with focus returning to what had it before the palette opened when the dialog closes,
and SHALL open the task's page otherwise.

#### Scenario: Dialog mode

- **GIVEN** Task opens in is Dialog and the reader is on the task list
- **WHEN** they choose a task in the palette
- **THEN** the task opens in the dialog over the list

#### Scenario: Page mode

- **GIVEN** Task opens in is Page
- **WHEN** they choose a task in the palette
- **THEN** the browser goes to the task's page

### Requirement: Recently used items come first

The palette SHALL remember the last eight items chosen in this browser and SHALL list them first,
most recent first, under Recent when nothing is typed, and rank them higher when something is.
When the page offers `tix.recentTasks()`, the tasks it names SHALL come first; when it does not,
the palette SHALL work the same without them.

#### Scenario: A screen chosen before

- **GIVEN** the reader chose Settings from the palette
- **WHEN** they open the palette again
- **THEN** Settings is the first option

#### Scenario: Recently viewed tasks

- **GIVEN** the page offers recently viewed tasks
- **WHEN** the reader opens the palette
- **THEN** those tasks are listed first
