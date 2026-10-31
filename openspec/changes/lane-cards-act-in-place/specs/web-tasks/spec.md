## RENAMED Requirements

- FROM: `### Requirement: A lane card offers a tick box only for one unambiguous completion`
- TO: `### Requirement: A lane card's tick completes and reopens along its workflow`

## MODIFIED Requirements

### Requirement: A lane card's tick completes and reopens along its workflow

A lane card's tick SHALL complete an unfinished task and reopen a finished one, following the task's
own project's workflow. For an unfinished task the candidate routes SHALL be the shortest routes from
its state to each terminal state of the done category. For a task in a terminal state the candidate
routes SHALL be the routes to non-terminal states that have the fewest steps.

A candidate SHALL be usable only when no step of it requires a comment and the reader holds the
scope to transition tasks and every scope a step requires. When there is exactly one candidate and
it is usable, the tick SHALL be a box that applies it. When there is more than one candidate, the
tick SHALL open a menu offering every usable candidate and naming the states each passes through.
When no candidate is usable, or there is none, or the reader cannot transition tasks, the tick SHALL
open a panel that says why in visible text and links to the task's own screen.

Every tick SHALL submit its route to the task's transition route as a form post carrying the CSRF
token and the listing to return to, and SHALL work without scripting. Where a route passes through
other states the control SHALL name them in its accessible name before it is applied. A finished
card SHALL render ticked whichever control it offers.

#### Scenario: Ticking completes the task along its only route

- **GIVEN** a task in `todo` in the built-in workflow
- **WHEN** the reader ticks its box in the lanes view
- **THEN** the task moves through `doing` to `done`
- **AND** the reader returns to the same listing

#### Scenario: Unticking reopens the task along its nearest route

- **GIVEN** a task in `done` in the built-in workflow
- **WHEN** the reader ticks its box again
- **THEN** the task moves to `todo`

#### Scenario: Two equally near open states offer the choice

- **GIVEN** a workflow in which two non-terminal states are each one step from a terminal state
- **WHEN** the lane card of a task in that terminal state is drawn
- **THEN** the tick opens a menu offering both routes and applies none by itself

#### Scenario: Two done states offer the choice

- **GIVEN** a workflow in which two terminal done-category states are reachable from the task's state
- **WHEN** the lane card is drawn
- **THEN** the tick opens a menu offering both routes

#### Scenario: A route that needs a comment says so

- **GIVEN** a workflow whose only route to done requires a comment
- **WHEN** the lane card is drawn
- **THEN** no route is submitted by the tick
- **AND** its panel says a comment is needed and links to the task

#### Scenario: A workflow with no way back says so

- **GIVEN** a workflow with no transition out of its done state
- **WHEN** the lane card of a finished task is drawn
- **THEN** the card renders ticked and its panel says no route leads back to an open state

#### Scenario: A reader without the scope is not offered a route

- **GIVEN** a reader who cannot transition tasks
- **WHEN** the lane card is drawn
- **THEN** no route is submitted by the tick and its panel says they cannot move tasks

### Requirement: Lanes stay current and fit the screen

The lanes view SHALL refresh from the event stream through the same listing fragment as the list and
the board, keeping the view, filter, sort, page and visibility choice. From 640px wide, lanes SHALL
scroll sideways inside their own frame rather than scrolling the page. Below 640px each lane SHALL
take most of the width of the screen, the lanes SHALL snap sideways one lane at a time, and each lane
SHALL be its natural height so that the page, not the lane, scrolls vertically. On a coarse pointer
the tick, status and priority controls of a lane card SHALL be at least 40px in both dimensions.

#### Scenario: A refresh keeps the lanes

- **GIVEN** a reader on the lanes view
- **WHEN** the listing fragment is fetched for a live refresh
- **THEN** it draws lanes holding the tasks on screen

#### Scenario: A phone scrolls the page, not the lane

- **GIVEN** a viewport 390px wide
- **WHEN** the lanes are drawn
- **THEN** a lane does not scroll vertically inside itself and the lanes snap sideways

## ADDED Requirements

### Requirement: A lane card's status and priority are menus

A lane card SHALL draw its status badge as a control that opens a menu of the routes the reader may
take from the task's state in its own project's workflow, omitting any route that needs a comment or
a scope the reader lacks, each submitted to the task's transition route. A reader who cannot
transition tasks, or a card with no route to offer, SHALL see the badge as a label.

A lane card SHALL draw its priority as a control that opens a menu of every priority, marking the
current one, each submitted to the task's priority route, for a reader who may write tasks. A reader
who may not SHALL see the priority as a label, and a normal priority SHALL then not be drawn.

Both menus SHALL work as plain forms without scripting and SHALL be operable from the keyboard.

#### Scenario: The status menu offers the workflow's moves

- **GIVEN** a task in `todo` in the built-in workflow and a reader who can transition tasks
- **WHEN** the reader opens the card's status menu
- **THEN** it offers `doing`, `blocked` and `cancelled` and the route through `doing` to `done`
- **AND** choosing `blocked` moves the task to `blocked`

#### Scenario: A move needing a comment is left out

- **GIVEN** a workflow whose transition into `blocked` requires a comment
- **WHEN** the card's status menu is drawn
- **THEN** it does not offer `blocked`

#### Scenario: The priority menu changes the priority

- **GIVEN** a reader who may write tasks
- **WHEN** they choose `high` in a card's priority menu
- **THEN** the task's priority is high

#### Scenario: A reader who may not change them sees labels

- **GIVEN** a reader who may only read tasks
- **WHEN** the lanes are drawn
- **THEN** no card carries a status or priority form

### Requirement: Lane actions change only their own card

With scripting, every tick, status and priority action on a lane card SHALL replace only that card
with its newly drawn self, SHALL mark the card busy while the request is in flight, and SHALL return
keyboard focus to the same control on the new card. Markup a live refresh draws SHALL behave as the
markup it replaced, so a lane action after a refresh is still swapped in place. A refused action SHALL leave the card in place and
show the refusal inside it. A live refresh SHALL NOT replace the listing while a lane action is in
flight or a menu in it is open, and SHALL return focus to the element that held it when it lands.

#### Scenario: A tick swaps one card

- **GIVEN** a reader on the lanes view with scripting
- **WHEN** they tick a card
- **THEN** only that card is replaced and focus is on its tick

#### Scenario: A refusal is shown on the card

- **GIVEN** a card whose task another reader has just moved where the route no longer applies
- **WHEN** the reader ticks it
- **THEN** the card stays and says why the move was refused

#### Scenario: A refresh waits for an open menu

- **GIVEN** a reader with a lane card's status menu open
- **WHEN** a live update arrives
- **THEN** the listing is not replaced until the menu closes

### Requirement: A focused button keeps its own keys

A keyboard shortcut SHALL NOT take Enter or Space from a focused button or summary, or Enter from a
focused link, so that every control on a page can be pressed from the keyboard whatever scheme is
active. While a menu is open no shortcut SHALL act, so Escape closes the menu and nothing else.

#### Scenario: Enter presses a focused lane control

- **GIVEN** the default shortcut scheme, which binds Enter to opening the selected row
- **WHEN** the reader focuses a lane card's status badge and presses Enter
- **THEN** the status menu opens

#### Scenario: Escape closes a menu and stays on the page

- **GIVEN** a lane card's status menu is open
- **WHEN** the reader presses Escape
- **THEN** the menu closes and the page does not go back
