## MODIFIED Requirements

### Requirement: Selecting rows on the task list

Rows of the web task list, board cards and lane cards SHALL carry a selection checkbox shown only in
select mode, labelled "Select" and the task's key. With scripting on, shift-click SHALL select the
range from the last box clicked, `x` SHALL toggle the focused row's box, Ctrl or Cmd+A SHALL select
every row while focus is in the list, and a live refresh SHALL wait while any row is selected.

#### Scenario: Range selection

- **GIVEN** select mode is on
- **WHEN** a reader ticks the second row and shift-clicks the fifth
- **THEN** rows two to five are selected and the bar reads "4 tasks selected"

#### Scenario: Escape clears

- **WHEN** three rows are selected and the reader presses Escape
- **THEN** no row is selected, select mode is off and the page does not navigate back

#### Scenario: Board and lanes select too

- **WHEN** a reader opens the board or the lanes with `?select=1`
- **THEN** every card carries a box labelled "Select KEY-N" that belongs to the bulk bar's form

### Requirement: The bulk action bar

In select mode the task screen SHALL show a sticky bar naming how many tasks are selected, announced
as a status, and offering: move to a status, priority, add tag, remove tag, assignee, star, unstar,
copy to a project, delete and Cancel. Each action SHALL be one request for the whole selection, after
which the screen SHALL show a toast naming how many tasks changed and each refused task with its
reason.

#### Scenario: Bulk move with a refusal

- **WHEN** a reader selects OPS-1 and WEB-2 and moves them to `done`, which WEB-2's workflow cannot reach
- **THEN** OPS-1 is done and the toast says one task moved and names WEB-2 with the reason

#### Scenario: No bar outside the mode

- **WHEN** a reader opens the task list without `select=1` and has not entered select mode
- **THEN** neither the bar nor any selection box is shown

## ADDED Requirements

### Requirement: Rows show one done control by default

Outside select mode a task row SHALL show the round done control and no selection checkbox. In
select mode the done control SHALL stay, keep its "Complete KEY-N" label and be dimmed, and the
square selection checkbox SHALL be shown beside it.

#### Scenario: Default row

- **WHEN** a reader opens the task list
- **THEN** each open row shows the round done control and no square

#### Scenario: Select mode row

- **WHEN** select mode is on
- **THEN** each row shows the square checkbox and a dimmed round done control

### Requirement: The Select toggle

The task screen's toolbar SHALL carry a Select toggle on the list, the board and the lanes whenever
there is a bulk action to offer, with `aria-pressed` saying whether select mode is on. Without
scripting it SHALL be a link to the same address with `select=1` added, or removed when on. The bar's
Cancel SHALL be the same link as the toggle when on.

#### Scenario: Turning it on without scripting

- **WHEN** a reader without scripting follows Select on `/tasks?q=tag:ops`
- **THEN** `/tasks?q=tag:ops&select=1` opens with the toggle pressed, the boxes and the bar shown

### Requirement: The select parameter is kept

The `select` parameter SHALL be kept by the view switch, the Group lanes by control, the pager, the
filter form, Clear and Show all, and SHALL be carried into the bulk bar's return address, so a bulk
action without scripting comes back in select mode.

#### Scenario: Bulk action without scripting

- **GIVEN** `/tasks?select=1`
- **WHEN** a reader without scripting ticks two rows and presses Star
- **THEN** both tasks are starred and the browser returns to `/tasks?select=1` with the toast

### Requirement: Entering select mode with scripting

With scripting on, select mode SHALL be entered by pressing the toggle, by `x` on a focused row (which
also selects it), by Shift-click or Ctrl or Cmd-click on a row's empty area, and by a long press of
about 500ms on a touch screen that does not scroll. A click on a link, button or field in the row
SHALL keep its own meaning.

#### Scenario: x enters the mode

- **WHEN** a reader focuses a row and presses `x`
- **THEN** select mode is on and that row is selected

#### Scenario: Long press

- **WHEN** a reader holds a finger on a row for half a second without moving it
- **THEN** select mode is on, the row is selected, and the release does not open the task

### Requirement: Leaving select mode

Select mode SHALL end, clearing the selection, on Escape when no menu, dialog or field holds it, on
the bar's Cancel, on the toggle pressed again, and when the reader unselects the last selected task.

#### Scenario: Cancel leaves

- **WHEN** select mode is on and the reader presses Cancel in the bar
- **THEN** no row is selected and no box or bar is shown

#### Scenario: Last one unselected

- **WHEN** one row is selected and the reader unticks it
- **THEN** select mode is off

### Requirement: Selection keys

The shortcut table SHALL bind Shift+Up and Shift+Down to extend the selection from the focused row
and `d` to mark the focused task done, in every scheme, and the `?` overlay SHALL list them.

#### Scenario: Extending

- **WHEN** a reader focuses the second row and presses Shift+Down twice
- **THEN** rows two to four are selected and the fourth has focus

#### Scenario: Done from the keyboard

- **WHEN** a reader focuses an open row and presses `d`
- **THEN** the row's done control is pressed

### Requirement: Swaps keep the mode and the selection

A live refresh, a row's own swap and the in-place filter SHALL leave select mode as it was, and a
row redrawn while selected SHALL come back selected.

#### Scenario: Ticking a selected row done

- **GIVEN** select mode with INFRA-2 selected
- **WHEN** the reader marks INFRA-2 done and its row is swapped
- **THEN** select mode is still on and INFRA-2 is still selected
