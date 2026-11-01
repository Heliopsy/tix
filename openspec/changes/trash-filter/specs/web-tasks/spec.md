## ADDED Requirements

### Requirement: The browser has a trash

The sidebar SHALL carry a Trash entry linking to the task list filtered by `is:trashed`, marked
current while that listing is shown, with Tasks not marked current at the same time. That listing
SHALL be titled Trash, SHALL say that its rows are deleted tasks, SHALL be drawn as the list whatever
view the reader chose, SHALL NOT narrow to open work, and SHALL NOT offer the quick-add row.

#### Scenario: Opening the trash

- **GIVEN** a tenant with a deleted task
- **WHEN** the reader follows Trash in the sidebar
- **THEN** the screen is titled Trash, lists the deleted task and no live task, and Trash is the current entry

#### Scenario: The trash ignores the board preference

- **GIVEN** a reader whose task screen is drawn as the board
- **WHEN** they open the trash
- **THEN** the deleted tasks are drawn as list rows

### Requirement: A deleted row offers Restore

A task row for a soft-deleted task SHALL be marked deleted, SHALL NOT offer the complete tick, the
status and priority menus or a link to the task screen, and SHALL offer a Restore button posting to the task's restore route with
the listing as the place to return to.

#### Scenario: Restoring from the trash

- **GIVEN** the trash listing two deleted tasks
- **WHEN** the reader presses Restore on one
- **THEN** that task is live again, the browser returns to the trash with a "task restored" message, and only the other task is listed
