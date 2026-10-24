# web-ui

## ADDED Requirements

### Requirement: The task list opens on unfinished work

With no filter asked for, the task list SHALL show only tasks in states the
project's workflow does not treat as terminal. It SHALL say on screen that the
listing is narrowed and SHALL offer a single control that shows everything.

#### Scenario: A list with finished work in it

- **WHEN** a reader opens the task list without asking for a filter, in a
  tenant holding both open and finished tasks
- **THEN** only the unfinished tasks are listed, the summary says the listing
  is open work only, and a link shows the finished work too

#### Scenario: Asking for everything

- **WHEN** that reader follows the link
- **THEN** finished tasks are listed as well, and the summary counts both

#### Scenario: Everything is finished

- **WHEN** every task in the listing is in a terminal state
- **THEN** the summary reads as nothing open rather than as nothing on the
  list, because a project whose work is finished is not a project with no work

#### Scenario: A workflow that renamed its terminal state

- **WHEN** the tenant's workflow calls its terminal state something other than
  the built-in name
- **THEN** that state is still excluded from the default listing

#### Scenario: Paging through everything

- **WHEN** a reader who has asked for finished work follows the next-page
  control
- **THEN** the following page still lists finished work, because the position
  it addresses was walked over that same set

#### Scenario: Following the control from a listing the reader built

- **WHEN** a reader follows the control from a listing carrying a sort, a page
  size or a deadline window
- **THEN** the listing of everything keeps all three

### Requirement: The board is not narrowed to the open states

The board view SHALL draw every state its workflow declares, terminal states
included, whatever the list view's default is. Its columns are the workflow,
so a default that empties the terminal ones removes the only place a finished
card can be drawn.

#### Scenario: Finishing a card

- **WHEN** a reader on the board moves a card into a terminal state
- **THEN** the card is drawn in that state's column

#### Scenario: The board says nothing about a narrowing that did not happen

- **WHEN** a reader opens the board without asking for a filter
- **THEN** the screen does not report the listing as open work only

### Requirement: A finished task carries no deadline

A task in a terminal state SHALL NOT be drawn as overdue or due soon on any
surface.

#### Scenario: A task finished after its due date

- **WHEN** a task whose due date has passed is moved to a terminal state
- **THEN** it carries no due badge

#### Scenario: Reopening it

- **WHEN** that task is moved back out of the terminal state
- **THEN** its deadline is compared to the present again

### Requirement: Navigation offers only what the reader can reach

The configuration and data navigation groups SHALL be shown only to a reader
holding a scope that makes at least one screen in them reachable.

#### Scenario: A member refused at an administration screen

- **WHEN** a reader without any administration scope requests one of those
  screens and is refused
- **THEN** the refusal page does not offer the administration navigation

#### Scenario: A reader who can use one of those screens

- **WHEN** a reader holding the scope one of those screens is gated on, and no
  other administration scope, opens that screen
- **THEN** the page offers the navigation group the screen belongs to
