## ADDED Requirements

### Requirement: The heading counts the whole selection and names dependency waits

The task screen's summary SHALL count every task the filter selects, not the page drawn. Tasks with an
unfinished dependency SHALL be counted as "N waiting on other tasks", linking to the same view with
`is:waiting-on` added.

#### Scenario: Counts beyond one page

- **GIVEN** 30 open tasks and a page size of 10
- **WHEN** the reader opens page 1 of the list
- **THEN** the heading says 30 to go

#### Scenario: The waiting count links to the filtered view

- **GIVEN** a task depending on an unfinished task
- **WHEN** the reader opens `/tasks`
- **THEN** the heading says "1 waiting on another task" and links to `/tasks?q=is:waiting-on`

### Requirement: Waiting cards are marked

Every board card, lane card and list row whose task waits on an unfinished dependency SHALL carry a mark
naming the dependency ("waiting on REF") or the count of them, with every open dependency in its title
and a link to the first.

#### Scenario: A row waiting on one task

- **GIVEN** INFRA-2 depends on the unfinished INFRA-1
- **WHEN** the reader opens the list
- **THEN** INFRA-2's row carries "waiting on INFRA-1" linking to `/tasks/INFRA-1`

### Requirement: Board columns are not paged together

The board SHALL read each column separately, draw up to 50 cards a step, count the column's whole state
in its heading, and offer Show N more for that column alone. The lanes view SHALL NOT render the pager.

#### Scenario: A column on a busy board

- **GIVEN** 60 tasks in todo and 2 in doing
- **WHEN** the reader opens the board
- **THEN** the todo heading says (60), it draws 50 cards and a Show 10 more link, and doing draws both its cards
