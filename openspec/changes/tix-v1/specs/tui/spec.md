## ADDED Requirements

### Requirement: Terminal interface over the shared service interface

The TUI SHALL perform every operation through the same service interface the CLI and the HTTP API use, so business rules, validation, and authorization behave identically to the other access paths.

#### Scenario: Validation matches the CLI

- **WHEN** an edit that the CLI rejects as invalid is attempted in the TUI
- **THEN** the TUI reports the same rejection

#### Scenario: Authorization matches the API

- **WHEN** an operation the caller is not authorized for is attempted in the TUI
- **THEN** it is refused with the same authorization error the API returns

### Requirement: Transport agnosticism

The TUI SHALL work identically against a local database file and against a remote server, with no behaviour that depends on which transport is in use.

#### Scenario: Same behaviour on both transports

- **WHEN** the same sequence of actions is performed against a local database and against a remote server holding equivalent data
- **THEN** the resulting state and the displayed results are equivalent

#### Scenario: Transport is chosen by configuration

- **WHEN** the TUI is started with a configuration selecting a remote server
- **THEN** it operates against that server without any transport-specific option being required

#### Scenario: Zero configuration starts locally

- **WHEN** the TUI is started with no configuration present
- **THEN** it opens against the default local database and is immediately usable

### Requirement: Project selection view

The TUI SHALL provide a view listing the projects the caller can access, and selecting a project SHALL open its board.

#### Scenario: Projects are listed

- **WHEN** the project view is opened
- **THEN** the accessible projects are listed

#### Scenario: Selection opens the board

- **WHEN** a project is selected
- **THEN** that project's board is displayed

#### Scenario: No projects is not an error

- **WHEN** the caller has access to no projects
- **THEN** an empty state is displayed explaining how to create one, and the program keeps running

### Requirement: Board grouped by workflow state

The TUI SHALL display a board with one column per state of the selected project's workflow, showing the tasks in each state in a defined order.

#### Scenario: Columns follow the workflow

- **WHEN** a project's workflow declares four states
- **THEN** the board shows those four states in the workflow's declared order

#### Scenario: Tasks appear in their state

- **WHEN** a task is in a given workflow state
- **THEN** it appears in that state's column

#### Scenario: Ordering is stable

- **WHEN** the board is redrawn with no change in the underlying data
- **THEN** the tasks appear in the same order as before

### Requirement: Task detail view

The TUI SHALL provide a task detail view showing the task's built-in fields, its custom fields, its subtasks, its dependencies, and its comments, and SHALL allow returning to the board from it.

#### Scenario: Detail shows the task

- **WHEN** a task is opened from the board
- **THEN** its fields, custom fields, subtasks, dependencies, and comments are displayed

#### Scenario: Return to the board

- **WHEN** the user leaves the detail view
- **THEN** the board is displayed again with the previous selection retained

### Requirement: Filter and search sharing the CLI grammar

The TUI SHALL provide a filter and search input that accepts the same filter grammar as the CLI, and an expression valid in the CLI SHALL select the same tasks in the TUI.

#### Scenario: CLI expression selects the same tasks

- **WHEN** a filter expression that selects a set of tasks in the CLI is entered in the TUI
- **THEN** the same tasks are shown

#### Scenario: Invalid expression is explained

- **WHEN** a malformed expression is entered
- **THEN** the parse error is displayed and the previous result set remains until a valid expression is entered

#### Scenario: Filter applies to the board

- **WHEN** a filter is active
- **THEN** each column shows only the tasks matching the filter

### Requirement: Keyboard navigation with discoverable keybindings

The TUI SHALL be fully operable from the keyboard, and the keybindings available in the current view SHALL be discoverable within the interface rather than only in external documentation.

#### Scenario: Every action has a key

- **WHEN** any action offered by the current view is required
- **THEN** it can be performed with the keyboard alone

#### Scenario: Bindings are shown

- **WHEN** a view is displayed
- **THEN** the keys for its primary actions are indicated in the interface

#### Scenario: Unknown key is harmless

- **WHEN** a key with no binding in the current view is pressed
- **THEN** nothing changes and no error state is entered

### Requirement: In-app help view

The TUI SHALL provide a help view listing the available keybindings and their meanings, reachable from any view, and dismissible back to the previous view.

#### Scenario: Help is reachable

- **WHEN** the help key is pressed from any view
- **THEN** the help view is displayed

#### Scenario: Help is contextual and complete

- **WHEN** the help view is displayed
- **THEN** it lists the bindings for the view it was opened from as well as the global bindings

#### Scenario: Help is dismissible

- **WHEN** the help view is dismissed
- **THEN** the previous view is restored with its state intact

### Requirement: Live updates from the event stream

The TUI SHALL subscribe to the event stream and reflect changes made elsewhere without the user performing a manual refresh.

#### Scenario: External change appears

- **WHEN** another client changes a task's state while the board is displayed
- **THEN** the board reflects the new state without user action

#### Scenario: Selection survives an update

- **WHEN** the board updates from an incoming event
- **THEN** the user's current selection and scroll position are preserved where the selected task still exists

#### Scenario: Reconnect resumes without gaps

- **WHEN** the event subscription drops and reconnects
- **THEN** changes that occurred while disconnected are applied and the displayed state is correct

### Requirement: Claim, release, and transition from the board

The TUI SHALL allow a task to be claimed, released, and transitioned directly from the board, and these actions SHALL obey the same claim, lease, and workflow rules as the other access paths.

#### Scenario: Claim from the board

- **WHEN** the user claims an unclaimed task from the board
- **THEN** the task is claimed and the board shows it as claimed

#### Scenario: Losing a claim race is reported

- **WHEN** the user attempts to claim a task that another worker has already claimed with a live lease
- **THEN** the attempt is refused with a message explaining the task is already claimed, and the board reflects the current owner

#### Scenario: Illegal transition is refused

- **WHEN** the user attempts a transition the workflow does not permit
- **THEN** the transition is refused with an explanation and the task is unchanged

#### Scenario: Release returns the task

- **WHEN** the user releases a task they hold
- **THEN** the task becomes available to other workers and the board reflects that

### Requirement: Graceful degradation on narrow terminals

The TUI SHALL remain usable when the terminal is too narrow to show every column at full width, by adapting the layout rather than producing broken or truncated output that hides content.

#### Scenario: Narrow terminal adapts

- **WHEN** the terminal is narrower than the full board layout requires
- **THEN** the layout adapts so that columns remain reachable and task titles remain readable

#### Scenario: Resize is handled

- **WHEN** the terminal is resized while the TUI is running
- **THEN** the layout redraws to fit the new size without corrupting the display

#### Scenario: Very small terminal explains itself

- **WHEN** the terminal is smaller than any usable layout
- **THEN** a message stating the minimum usable size is displayed instead of a corrupted board

### Requirement: Colour support and NO_COLOR

The TUI SHALL render correctly on terminals without colour support, and SHALL disable colour output when the NO_COLOR environment variable is set, conveying state through text or symbols rather than colour alone.

#### Scenario: NO_COLOR disables colour

- **WHEN** NO_COLOR is set and the TUI starts
- **THEN** no colour escape sequences are emitted and every view remains legible

#### Scenario: State is not conveyed by colour alone

- **WHEN** colour is unavailable
- **THEN** task state, claimed status, and selection remain distinguishable through text or symbols

### Requirement: Clean exit restoring the terminal

The TUI SHALL restore the terminal to its prior state on exit, including when exiting through an interrupt signal, leaving no altered screen mode, hidden cursor, or residual input mode.

#### Scenario: Normal quit restores the terminal

- **WHEN** the user quits the TUI
- **THEN** the terminal returns to its previous screen and input mode with the cursor visible

#### Scenario: Interrupt restores the terminal

- **WHEN** the TUI receives an interrupt signal
- **THEN** it exits and the terminal is restored to its previous state

#### Scenario: Exit status reflects the outcome

- **WHEN** the TUI exits normally and when it exits because of a fatal error
- **THEN** the exit statuses differ and indicate success or failure

### Requirement: Errors surfaced rather than crashing

Errors from the service, the transport, or the event subscription SHALL be displayed within the interface, and the program SHALL remain running and usable where the error is recoverable.

#### Scenario: Service error is displayed

- **WHEN** an operation fails with a service error
- **THEN** the message is shown in the interface and the TUI remains usable

#### Scenario: Connection loss is reported

- **WHEN** the connection to a remote server is lost
- **THEN** the TUI indicates the disconnected state and continues running while attempting to reconnect

#### Scenario: Unrecoverable error exits cleanly

- **WHEN** an error occurs from which the TUI cannot continue
- **THEN** the terminal is restored, the error is printed, and a failing exit status is returned
