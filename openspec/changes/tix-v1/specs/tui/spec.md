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

### Requirement: Task editing from the terminal interface

The TUI SHALL allow a task to be created, retitled, re-bodied, reprioritised, reassigned, commented on, tagged, untagged, and given a dependency, and SHALL perform each through the same service call the CLI and the HTTP API use, holding no validation of its own.

#### Scenario: A task is created from the board

- **WHEN** the user creates a task from an open board
- **THEN** the task is created in that project and appears on the board

#### Scenario: A field is edited in place

- **WHEN** the user edits the title, body, priority, or assignee of the selected task
- **THEN** the change is applied and the board and the detail view show the new value

#### Scenario: A comment is recorded

- **WHEN** the user comments on the selected task
- **THEN** the comment is recorded and appears in the task's comment thread

#### Scenario: A tag is attached and detached

- **WHEN** the user adds a tag to the selected task and later removes it
- **THEN** the task carries the tag and then does not

#### Scenario: A dependency is added by reference

- **WHEN** the user names another task by the reference form the CLI accepts
- **THEN** the dependency is recorded against the selected task

#### Scenario: A malformed reference is refused without reaching the service

- **WHEN** the user names a dependency that is not a valid task reference
- **THEN** the interface reports the parse failure and no call is made

#### Scenario: Validation is the service's, not the interface's

- **WHEN** an edit the service rejects is submitted
- **THEN** the service's own refusal is displayed and the task is unchanged

#### Scenario: An empty input cancels

- **WHEN** an input is accepted with nothing typed into it
- **THEN** the action is abandoned and no call is made

#### Scenario: The same actions are offered wherever a task is selected

- **WHEN** a task is selected on the board and when the same task is open in the detail view
- **THEN** the same editing actions are available in both

### Requirement: Navigation back out of a view

The TUI SHALL maintain a stack of the views the user has entered, and SHALL provide a binding that returns one level at a time, so no view can be entered that the user cannot leave without ending the program.

#### Scenario: Back returns one level

- **WHEN** the user has opened a board from the project list and a task from that board, and presses the back key twice
- **THEN** the board is shown and then the project list is shown

#### Scenario: Back at the top level does nothing

- **WHEN** the back key is pressed on the project list
- **THEN** the view does not change and the program keeps running

#### Scenario: Quit steps out of a nested view

- **WHEN** the quit key is pressed while a view is open above the project list
- **THEN** the interface returns one level rather than ending the program

#### Scenario: Quit at the top level ends the program

- **WHEN** the quit key is pressed on the project list
- **THEN** the program exits

#### Scenario: Cancelling an input does not leave the view

- **WHEN** an input or a picker is open and the cancel key is pressed
- **THEN** the input closes and the view it was opened over is still displayed

#### Scenario: Reloading a view does not lengthen the way back

- **WHEN** the open view is reloaded from the event stream or by a manual refresh
- **THEN** the number of levels between it and the project list is unchanged

#### Scenario: The way out is advertised

- **WHEN** any view is displayed
- **THEN** its footer names either the key that returns one level or the key that exits

### Requirement: Scrolling lists that do not fit the terminal

Every list the TUI displays SHALL scroll with its selection, and SHALL state that it continues beyond the visible rows, so no entry is unreachable on a short terminal.

#### Scenario: Every project is reachable on a short terminal

- **WHEN** the project list holds more projects than the terminal has rows and the selection is moved through it
- **THEN** every project is displayed at some point and can be opened

#### Scenario: A board column scrolls with its selection

- **WHEN** a column holds more tasks than the column has rows and the selection is moved down it
- **THEN** every task in the column is displayed at some point

#### Scenario: A list that continues says so

- **WHEN** a list holds more entries than are visible
- **THEN** the interface states how many entries lie above and below the visible rows

#### Scenario: A list that fits claims nothing

- **WHEN** every entry of a list is visible
- **THEN** no indication of further entries is shown

### Requirement: Empty states that explain themselves

Where the TUI has nothing to display it SHALL say so in words and SHALL say what would change that, rather than drawing an empty frame that cannot be told apart from a failure.

#### Scenario: An empty board says it is empty

- **WHEN** a project's board holds no tasks
- **THEN** the interface states that the board is empty and names the key that adds a task to it

#### Scenario: A filtered-out board blames the filter

- **WHEN** a board holds tasks but the active filter excludes all of them
- **THEN** the interface states that no task matches the filter and names the keys that clear or change it

#### Scenario: A project with no workflow says so

- **WHEN** a project's workflow declares no states
- **THEN** the interface states that there is no board and names the command that defines a workflow

#### Scenario: An empty project list says how to create one

- **WHEN** the caller can reach no projects
- **THEN** the interface states that there are none and gives the command that creates one

### Requirement: Colour vocabulary shared with the CLI

The TUI SHALL colour a workflow state by the category its state belongs to rather than by the state's name, SHALL distinguish the ends of the priority range, SHALL render a task reference distinctly, and SHALL use the same colours for these meanings as the CLI does.

#### Scenario: A state is coloured by its category

- **WHEN** a board column's state belongs to a known category
- **THEN** it is drawn in the colour the CLI draws that category in

#### Scenario: A user-defined state is not guessed at

- **WHEN** a state belongs to no known category
- **THEN** it is drawn without colour rather than assigned a guessed one

#### Scenario: The top of the queue stands out

- **WHEN** tasks of differing priority are drawn
- **THEN** the highest priority is distinguished and the lowest is muted, matching the CLI

#### Scenario: A project carries its own colour and icon

- **WHEN** a project has a palette colour and an icon
- **THEN** the project list and the board header render them

#### Scenario: Colour is never the only cue

- **WHEN** colour is unavailable
- **THEN** selection, claimed status, blocked status, and priority remain readable as text

#### Scenario: A destination that is not a terminal is not coloured

- **WHEN** the interface writes somewhere that is not a character device
- **THEN** no colour escape sequences are emitted, as the CLI does for the same destination

### Requirement: Terminal interface parity is enforced

The capability registry SHALL include the terminal interface among the surfaces every operation must reach, so an operation that binds no terminal view SHALL carry a recorded exemption stating why, and a missing binding SHALL be marked as a gap rather than passing unnoticed.

#### Scenario: A new operation cannot omit the terminal interface silently

- **WHEN** an operation declares no terminal binding and no terminal exemption
- **THEN** the parity tests fail

#### Scenario: A binding must name a view that exists

- **WHEN** an operation names a terminal view the interface does not offer
- **THEN** the parity tests fail

#### Scenario: Daily work is bound rather than exempted

- **WHEN** an operation a person performs on a board by hand carries a terminal exemption instead of a binding
- **THEN** the parity tests fail

#### Scenario: Remaining gaps are counted

- **WHEN** the number of terminal gaps changes
- **THEN** the parity tests fail until the recorded count is brought into line

### Requirement: Footer keys are a promise

The keys a view advertises on its footer SHALL be only those the selected task can accept, so a key shown to the user SHALL NOT be refused when pressed. The help view SHALL continue to document every binding the view has, whatever the current selection allows.

#### Scenario: An unclaimed task offers the claim key

- **WHEN** the selected task is unclaimed
- **THEN** the footer offers the claim key and offers neither release nor renew

#### Scenario: A task held here offers release and renew

- **WHEN** the selected task is held under a lease this session took
- **THEN** the footer offers release and renew and does not offer claim

#### Scenario: A task held elsewhere offers no lease key

- **WHEN** the selected task is held by another worker
- **THEN** the footer offers no lease key and the interface states who holds it

#### Scenario: A state with no way out does not offer a transition

- **WHEN** the workflow permits no transition from the selected task's state
- **THEN** the footer does not offer the transition key

#### Scenario: Help documents what the footer hides

- **WHEN** the help view is opened
- **THEN** it lists every binding the view has, including those the current selection cannot use

### Requirement: Keybinding schemes

The TUI SHALL offer named keybinding schemes and per-action overrides, SHALL provide a settings view for choosing between them, and SHALL render every advertised key from the active bindings rather than from fixed text.

#### Scenario: A scheme is chosen from the settings view

- **WHEN** the user opens the settings view and selects a scheme
- **THEN** the scheme takes effect immediately and the footer advertises its keys

#### Scenario: Every scheme leaves every action reachable

- **WHEN** any shipped scheme is active
- **THEN** every action carries at least one key and a description

#### Scenario: Advertised keys follow the active scheme

- **WHEN** two different schemes are active in turn
- **THEN** the keys named in the footer differ accordingly

#### Scenario: An override keeps the action's meaning

- **WHEN** a single action is rebound on top of a scheme
- **THEN** the action answers to the new key and is still described by its own words

#### Scenario: A colliding override is refused

- **WHEN** an override would make one key mean two things within one view
- **THEN** the interface reports which key collided, with which actions, in which view, and does not apply the override

#### Scenario: A scheme cannot name an action the interface does not have

- **WHEN** a shipped scheme's table rebinds an action the key map does not carry
- **THEN** building that scheme's bindings fails loudly rather than dropping the binding and leaving the action on its default key

#### Scenario: An unusable configuration is reported rather than ignored

- **WHEN** the configured scheme name is not one that ships, or a configured override cannot be applied
- **THEN** the interface starts on the default bindings and states why

### Requirement: Tenant switching from inside the interface

The TUI SHALL offer a tenant view that names the tenant the session is working in and accepts a tenant key to switch to, SHALL validate that key by opening a connection pinned to it and asking that connection who the actor is, and SHALL NOT present a list of tenants to choose from. A session opened without a way to dial another tenant SHALL say so and name the command that can.

#### Scenario: The tenant in force is stated

- **WHEN** the tenant view is open
- **THEN** it names the tenant the session is working in and the actor it is working as

#### Scenario: A key is typed rather than picked from a list

- **WHEN** the user asks to switch tenant
- **THEN** the interface takes a typed key, and says that no list exists because an actor belongs to one tenant and tenant listings are scoped to it

#### Scenario: A reachable tenant replaces the session

- **WHEN** a typed key names a tenant the actor can reach
- **THEN** the session works in that tenant, the project list, board, task detail and event tail loaded from the previous tenant are discarded, and the interface returns to the project list

#### Scenario: An unreachable tenant leaves the session alone

- **WHEN** a typed key cannot be dialled, or the tenant it names refuses the actor
- **THEN** the interface reports the key that could not be reached and the session keeps working in the tenant it was in

#### Scenario: The key the session is already on is refused

- **WHEN** the typed key is the tenant already in force
- **THEN** the interface says so and dials nothing

#### Scenario: Leases are not silently abandoned

- **WHEN** a switch happens while this session holds leases
- **THEN** the interface states that those leases were dropped from the session and will expire unreleased, because a lease is held in the tenant it was taken in

#### Scenario: The previous tenant's events are not drawn under the new one

- **WHEN** an event from the subscription opened on the previous tenant arrives after a switch
- **THEN** it is discarded rather than recorded in the new tenant's activity tail

#### Scenario: A session that cannot switch says so

- **WHEN** the interface was started without a way to open another tenant
- **THEN** the tenant view states that this session cannot switch and names `tix tenant use`

### Requirement: Activity filtering in the terminal interface

The TUI SHALL offer a filter over its activity view, SHALL accept the same expression `tix audit ls --filter` accepts, and SHALL refuse by name any term the live event tail cannot answer rather than applying it as a term that matches nothing.

#### Scenario: The tail is narrowed

- **WHEN** an activity filter expression is applied
- **THEN** only the events it accepts are drawn, and the events it hides remain in the tail rather than being discarded

#### Scenario: The filter is the shared one

- **WHEN** the same expression is given to the activity view and to `tix audit ls --filter`
- **THEN** both are parsed by one parser, so a key accepted by one is accepted by the other

#### Scenario: A term an event cannot answer is refused

- **WHEN** an expression carries a `source:` term
- **THEN** the interface refuses it, states that an event carries no source, and names `tix audit ls` as where source can be filtered

#### Scenario: A bad expression keeps the one in force

- **WHEN** an expression cannot be parsed
- **THEN** the error is shown and the filter already applied keeps selecting

#### Scenario: The view says how much it is hiding

- **WHEN** a filter is in force
- **THEN** the status bar counts the events it keeps against the events the tail holds

#### Scenario: A filter that hides everything blames itself

- **WHEN** a filter accepts none of the events in the tail
- **THEN** the view says no event matches the filter, rather than saying the tenant is quiet

#### Scenario: The filter can be cleared

- **WHEN** the clear-filter key is pressed in the activity view
- **THEN** the whole tail is drawn again
