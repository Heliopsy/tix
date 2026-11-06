## ADDED Requirements

### Requirement: Selecting rows on the task list

Each row of the web task list SHALL carry a selection checkbox. With scripting on, shift-click
SHALL select the range from the last box clicked, `x` SHALL toggle the focused row's box,
Ctrl or Cmd+A SHALL select every row while focus is in the list, and Escape SHALL clear the
selection before it does anything else. A live refresh SHALL wait while any row is selected.

#### Scenario: Range selection

- **WHEN** a reader ticks the second row and shift-clicks the fifth
- **THEN** rows two to five are selected and the bar reads "4 selected"

#### Scenario: Escape clears

- **WHEN** three rows are selected and the reader presses Escape
- **THEN** no row is selected and the page does not navigate back

### Requirement: The bulk action bar

While rows are selected the task list SHALL show a sticky bar naming how many are selected and
offering: move to a status, priority, add tag, remove tag, assignee, star, unstar, copy to a
project, delete and clear. Each action SHALL be one request for the whole selection, after which
the list SHALL show a toast naming how many tasks changed and listing each refused task with its
reason. With scripting off the bar SHALL be drawn under the list and work the same way.

#### Scenario: Bulk move with a refusal

- **WHEN** a reader selects OPS-1 and WEB-2 and moves them to `done`, which WEB-2's workflow cannot reach
- **THEN** OPS-1 is done and the toast says one task moved and names WEB-2 with the reason

### Requirement: One Undo for a bulk delete

A bulk delete SHALL be soft and its toast SHALL offer one Undo that restores every task the
delete removed, in one request.

#### Scenario: Undo a bulk delete

- **WHEN** a reader deletes three selected tasks and presses Undo on the toast
- **THEN** all three tasks are back on the list

### Requirement: Export the list as CSV

The View menu of the task list SHALL offer Export CSV, which downloads every task the current
filter, deadline window, sort and project visibility select, up to 10000 rows, with the title
and the optional columns the reader's column picker shows, escaped by RFC 4180 and with the
formula guard.

#### Scenario: Export follows the column picker

- **WHEN** a reader has hidden the Tags column and exports the list filtered by `tag:ops`
- **THEN** the download holds only tasks tagged ops and has no tags column
