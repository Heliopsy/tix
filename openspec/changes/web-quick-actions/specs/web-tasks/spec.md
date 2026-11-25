## ADDED Requirements

### Requirement: A list row has an actions menu

Every list row of a task that is not deleted SHALL carry a More actions button opening a popover with:
open the task, complete or reopen, star or unstar, the moves its workflow allows, snooze or unsnooze, the
assignee, Assign to me, the due date, tags, and Select. Each SHALL be a form or link that works without
scripting, offered only to a reader with the scope it needs.

#### Scenario: A row's menu

- **WHEN** a reader with task write opens `/tasks`
- **THEN** each row's menu holds forms posting to its complete, star, transition, snooze, assign, due and tags routes, and a Select link to select mode

### Requirement: Assign to me in one press

The task screen's actions menu, a list row's menu and a lane card's menu SHALL offer Assign to me while the
task is not assigned to the reader, posting `assignee=me` to `/tasks/REF/assign`. The bulk bar SHALL offer
Assign to me, assigning every selected task to the reader.

#### Scenario: Assigning a task to oneself

- **WHEN** a reader presses Assign to me on an unassigned task
- **THEN** the task is assigned to the reader and the toast says assigned to you with an Undo

### Requirement: A lane card sets the due date

A lane card's More actions menu SHALL hold a due date field posting to `/tasks/REF/due`, reading what the
task screen's Due field reads, and a Clear button while the task has a due date.

#### Scenario: Setting a due date from a lane card

- **WHEN** a reader enters 2026-12-01 in a lane card's due date field and saves
- **THEN** the task is due on 2026-12-01 and only the card and the toast are redrawn

### Requirement: Every quick action offers an Undo

After a star, unstar, snooze, unsnooze, assignee change, due date change, tag add or tag remove, the toast
SHALL offer Undo, a form posting the previous value back to the matching route. An action that changed
nothing SHALL offer no Undo.

#### Scenario: Undoing an assign

- **WHEN** a reader assigns an unassigned task to themselves and presses Undo
- **THEN** the task is unassigned again

#### Scenario: Undoing an unsnooze

- **WHEN** a reader unsnoozes a task snoozed until Friday 09:00 and presses Undo
- **THEN** the task is snoozed until Friday 09:00 again
