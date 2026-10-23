## ADDED Requirements

### Requirement: The task screens reflect a change made anywhere else

The task listing, the project board and a single task's screen SHALL update
from the tenant's event stream while they are open, without the reader
reloading, whatever surface the change was made on.

Each refresh SHALL be rendered by the server, from the same template blocks
the full page was rendered by. The browser SHALL NOT be sent rendered markup
over the event stream, and SHALL NOT hold a model of task state of its own.

#### Scenario: A transition made in the terminal reaches an open task screen

- **GIVEN** a task screen open in a browser
- **WHEN** the task is moved from todo to done over SSH
- **THEN** the screen's status reads done without the reader reloading

#### Scenario: A listing updates when a task it shows changes

- **GIVEN** a task list open in a browser
- **WHEN** an agent claims a task on that list
- **THEN** the row says so without the reader reloading

#### Scenario: A refreshed row is the row the page renders

- **WHEN** the fragment a refresh fetches and the page it belongs to both render the same task
- **THEN** the markup of that row is identical, because one template block produced both

#### Scenario: Without JavaScript the screens still work

- **WHEN** a task screen is opened with JavaScript disabled
- **THEN** it shows the current state on load and every control on it still works

### Requirement: A live refresh preserves what the reader is looking at

A refresh SHALL answer the same question the screen was already answering. The
filter, the deadline window, the sort, the page cursor and page size, the
list-or-board choice, the chosen columns, the project visibility choice and
the project or task in view SHALL all survive it.

#### Scenario: A filtered listing stays filtered

- **GIVEN** a task list narrowed by a filter expression and a sort
- **WHEN** an event refreshes it
- **THEN** the rows shown are the filtered, sorted rows, not the first page of everything

#### Scenario: A reader on page two stays on page two

- **GIVEN** a task list showing a page reached through its pager
- **WHEN** an event refreshes it
- **THEN** the same page of the listing is shown

#### Scenario: The board stays the board

- **GIVEN** a reader whose task screen is drawn as a board
- **WHEN** an event refreshes it
- **THEN** the board is redrawn, not the list

#### Scenario: A put-away project stays away

- **GIVEN** a task list with a project hidden by the visibility control
- **WHEN** an event refreshes it
- **THEN** that project's tasks are still absent

#### Scenario: A row action still returns to the screen

- **WHEN** a reader completes a task on a row that arrived through a refresh
- **THEN** they are returned to the listing they were on, filter and position intact, and never to the fragment route

### Requirement: A live refresh never discards work in progress

A refresh SHALL NOT replace a region in which the reader is working. A card
held in a drag, a field holding the caret, and a field changed away from what
the server rendered into it each SHALL defer the refresh. A deferred refresh
SHALL be applied once the region is idle, and SHALL NOT be dropped.

The forms that edit a task, its move control, its comment box and the
screen's own filter and quick-add controls SHALL NOT be inside a replaced
region at all.

#### Scenario: A refresh during a drag waits

- **GIVEN** a card being dragged between columns
- **WHEN** an event would refresh the board
- **THEN** the board is not replaced while the drag is in progress, and is refreshed once it ends

#### Scenario: A half-written comment survives

- **GIVEN** a reader part way through typing in a field inside a refreshed region
- **WHEN** an event would refresh it
- **THEN** what they typed is still there, and the refresh is applied after they stop

#### Scenario: The filter box is never swapped

- **WHEN** a task list is refreshed
- **THEN** the filter expression in the box and the text in the quick-add field are untouched

### Requirement: A screen subscribes only to what it can draw

Each screen SHALL subscribe with a filter naming the event types it renders,
and a screen about one project SHALL name that project. A screen about one
task SHALL further ignore an event that names neither that task nor a record
belonging to it.

#### Scenario: An unrelated tenant event changes nothing

- **GIVEN** an open task screen
- **WHEN** a webhook delivery or a sign-in is recorded for the tenant
- **THEN** the screen does not re-render

#### Scenario: A neighbouring task does not rebuild a task screen

- **GIVEN** a task screen open on one task
- **WHEN** another task in the same project is transitioned
- **THEN** that screen does not re-render

#### Scenario: A comment on the task does reach it

- **GIVEN** a task screen open on one task
- **WHEN** a comment is added to that task
- **THEN** the comment appears without the reader reloading
