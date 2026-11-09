## ADDED Requirements

### Requirement: The task screen's page survives the view switch

The view switch and the lanes grouping control SHALL keep the `page` parameter, while dropping a
cursor and its trail. A page number past the end of the target view SHALL open its last page.

#### Scenario: Switching to the board on page 2

- **GIVEN** a reader on page 2 of the task list
- **WHEN** they switch to the board
- **THEN** the board is drawn at page 2 and its pager says so

### Requirement: Changing the filter returns to page 1

The filter form, Clear, a project narrowing link and Show all SHALL NOT carry the page, so a changed
selection always opens on its first page.

#### Scenario: Applying a filter from page 3

- **GIVEN** a reader on page 3 of the task list
- **WHEN** they apply a different filter
- **THEN** the first page of the new selection is drawn

### Requirement: The live refresh keeps a numbered page

A live refresh of a numbered page SHALL redraw the same page number with the current total, and SHALL
NOT move the reader to another page unless the page no longer exists, when it SHALL draw the last one.

#### Scenario: A task is added while a reader is on page 2

- **GIVEN** a reader on page 2 of the task list
- **WHEN** another task is created and the screen refreshes
- **THEN** page 2 is drawn and the pager's total counts the new task
