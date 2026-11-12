## ADDED Requirements

### Requirement: Task opens in offers a panel beside the list

Task opens in SHALL offer Panel beside Page and Dialog, stored as tix_task_open=pane. With Panel, a task
followed from a listing on a window at least 1100px wide SHALL open in a panel on the right of the page while
the listing stays usable, its row marked aria-current, and its address SHALL be pushed so Back closes it.

#### Scenario: Opening a task beside the list

- **WHEN** a reader with Panel clicks a task's title in the task list on a 1440px window
- **THEN** the task screen opens beside the list, the row is marked current and the address is the task's

#### Scenario: Back closes the panel

- **WHEN** the panel is open and the reader presses Back or Escape inside it
- **THEN** the panel closes and the address is the list's again

### Requirement: The task panel has a resizable divider

The divider between the listing and the panel SHALL be a vertical separator announcing its width, dragged
with the pointer and moved with the arrow keys, Home, End and Enter. The width SHALL be at least 360px and at
most 720px or 60% of the room beside the sidebar, and kept per browser.

#### Scenario: Widening from the keyboard

- **WHEN** the divider has focus and the reader presses the left arrow
- **THEN** the panel is 16px wider, aria-valuenow says so and the width is kept for the next open

### Requirement: Panel falls back on a narrow window and without scripting

On a window narrower than 1100px, Panel SHALL open a task as Dialog does. Without scripting a task's title
SHALL stay a link to the task's own page whatever the preference.

#### Scenario: A phone with Panel chosen

- **WHEN** a reader with Panel opens a task on a 390px window
- **THEN** the task opens in the dialog, filling the screen
