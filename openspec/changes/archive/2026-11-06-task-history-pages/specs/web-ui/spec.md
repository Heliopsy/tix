## ADDED Requirements

### Requirement: Task history pages back to its first change

The task screen's History SHALL draw the newest changes, at most one page at the browser's configured
page size, and SHALL draw a Show older control under them while older changes exist. Following Show older
until it is gone SHALL draw every change exactly once, newest first. The last page SHALL NOT offer Show
older.

#### Scenario: A long history is walked to the start

- **GIVEN** a task with 120 audit entries and a page size of 25
- **WHEN** the reader opens the task and follows Show older until it is gone
- **THEN** all 120 changes have been drawn, newest first, none twice

#### Scenario: A history ending on a page boundary

- **GIVEN** a task with exactly two pages of audit entries
- **WHEN** the reader follows Show older once
- **THEN** the second page is drawn without a Show older control

### Requirement: Show older works with and without JavaScript

Show older SHALL be a link to the task screen at the older page, which opens History and offers a way
back to the newest changes. With JavaScript it SHALL fetch only the older rows and append them in place
of itself. A reader without the audit read scope SHALL be told so rather than handed rows.

#### Scenario: Without JavaScript

- **WHEN** a browser without JavaScript follows Show older
- **THEN** the task screen loads with History open on the older page and a Newest changes link

#### Scenario: A reader without audit scope

- **WHEN** a reader who may read the task but not the audit log fetches older history rows
- **THEN** the answer says they do not hold the audit read scope and carries no rows

### Requirement: A move is not split across history pages

A move that crosses several workflow states SHALL be drawn as one row even when its hops fall either side
of a page boundary: the older hops SHALL join the page holding the newer ones.

#### Scenario: A two-hop move on the boundary

- **GIVEN** a task whose 25th and 26th newest entries are the two hops of one completion
- **WHEN** the reader opens the task
- **THEN** the first page draws the completion as one row of two steps
- **AND** the next page draws neither hop
