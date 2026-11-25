## ADDED Requirements

### Requirement: Every view draws one task menu

The three dots on a lane card, a board card, a list row and the task screen or dialog SHALL open the same
menu, drawn by one template, with the same items in the same order and the same labels: Open the task;
Copy link and Copy reference; Complete or Reopen; Move; Priority; Assignee and Assign to me; Due date;
Tags; Rename; Snooze or Unsnooze; Star or Unstar; Copy to, Move to and Save as template; and Delete task
last, set apart and red.

#### Scenario: A list row and a lane card offer the same items

- **GIVEN** a reader with every scope and a task in the list and in the lanes
- **WHEN** they open the task's three dots in each view
- **THEN** both menus list the same items in the same order, the row adding only Select before Delete

#### Scenario: A board card's dots are the menu

- **WHEN** a board is drawn
- **THEN** every card carries the three dots in its corner opening the task menu, and no separate Move fold

### Requirement: The task menu is gated by scope the same way in every view

Complete, Reopen and Move SHALL need `task:transition`; Priority, Assignee, Due date, Tags, Rename,
Snooze, Copy to, Move to and Save as template SHALL need `task:write`; Delete SHALL need `task:delete`.
A reader who may only read SHALL be offered Open the task, the copy items and Star, in every view.

#### Scenario: A read-only reader

- **GIVEN** a reader with the viewer role
- **WHEN** they open the three dots on a lane card, a board card, a list row and the task screen
- **THEN** each menu offers Open the task (not on the task screen), Copy link, Copy reference and Star, and no form that changes the task

#### Scenario: A writer who may not delete

- **GIVEN** a reader who may write and move tasks but not delete them
- **WHEN** they open any view's task menu
- **THEN** it offers everything but Delete task

### Requirement: Only three task menu items depend on the view

The task screen SHALL leave out Open the task and SHALL add Reclaim the lease beside a live lease for a
holder of `task:reclaim`, and Also delete subtasks under a task that has subtasks. A list row SHALL add
Select just before Delete. A trash row SHALL use the same three dots with Restore and Delete permanently.

#### Scenario: Select is the list's

- **WHEN** a lane card, a board card and a list row are drawn for one task
- **THEN** only the list row's menu offers Select

### Requirement: Every task menu deletes through the one delete path

Delete task in every view SHALL post to the task's delete route with nothing but the place to return to,
so that with the trash on the task goes to the trash and the toast offers Undo, and with the trash off the
permanent-delete confirmation is drawn and nothing is deleted until it is confirmed.

#### Scenario: Deleting from a list row with the trash on

- **WHEN** a reader deletes a task from its list row's menu
- **THEN** the task leaves the list and the toast offers Undo

#### Scenario: Deleting from a list row with the trash off

- **GIVEN** a tenant whose trash is off
- **WHEN** a reader deletes a task from its list row's menu
- **THEN** the confirmation is drawn and the task is still there

### Requirement: Each task menu form swaps only its card or row

Each item SHALL be a form on an existing route or a link that works without scripting and returns to the
listing at the card or row, or to the task screen. With htmx a form SHALL swap only its own card or row and
the toast, put focus back on the control pressed, and show a refusal without the card or row disappearing.

#### Scenario: Starring from a board card's menu

- **WHEN** a reader stars a task from its board card's menu with scripting on
- **THEN** only that card and the toast are replaced

### Requirement: Dragging a board card offers the menu's moves

Dragging a board card SHALL light up and accept only the columns its menu's Move section offers in one
step, and the drop SHALL post that section's form with the column as the route.

#### Scenario: A column the reader may not reach

- **GIVEN** a card whose workflow has no move into a column
- **WHEN** the reader drags the card over that column
- **THEN** the column does not accept the drop and no request is made
