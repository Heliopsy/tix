# tui Specification

## Purpose

Provides the terminal interface, over the same service interface every other access path uses.

## Requirements

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

### Requirement: The terminal interface offers only the views a reader's authority reaches

The terminal interface SHALL offer a reader the views their scopes permit and SHALL NOT offer an entry to a
view whose every read the service would refuse. A view SHALL be considered reachable when the reader may
perform at least one of the read operations the capability registry binds to it.

The interface SHALL NOT hold its own table of the scopes a view requires. The set of reachable views SHALL
be supplied to it, resolved from the same scopes the registry records and the service enforces, and SHALL be
resolved once for the session rather than per keystroke.

A session supplied with no set of reachable views SHALL be offered only the views that read nothing from the
service.

#### Scenario: A reader who may not subscribe to events is not offered the activity view

- **WHEN** a session's actor holds the task and project read scopes but not the event subscribe scope
- **THEN** the activity view is not offered
- **AND** the board view is offered

#### Scenario: A demo visitor and an enrolled administrator are offered the same views

- **WHEN** one session's actor holds the sandbox visitor's scopes and another's holds the administrator role
- **THEN** both are offered the board, the detail, the activity, the statistics and the tenant views

#### Scenario: A reader who may only read projects is not offered the board

- **WHEN** a session's actor holds the project read scope alone
- **THEN** the board, the activity and the statistics views are not offered

#### Scenario: Pressing the key for a view that is not offered enters nothing

- **WHEN** a reader not offered a view presses the key that opens it
- **THEN** the open view does not change

#### Scenario: A session with no authority supplied reaches only the local views

- **WHEN** a session is built without a set of reachable views
- **THEN** only the project list, the settings screen and the help overlay are reachable

### Requirement: The help overlay agrees with what the interface offers

The help overlay SHALL list a cross-view binding only when the view it opens is offered to this reader, so
every key the overlay documents does what it says. Bindings that open no view SHALL always be listed.

#### Scenario: A refused view's key is absent from the overlay

- **WHEN** the overlay is opened by a reader not offered the activity view
- **THEN** the activity binding is not listed
- **AND** the help, quit, projects and statistics bindings are listed

#### Scenario: Every offered view's key is documented

- **WHEN** the overlay is opened by a reader offered every view
- **THEN** the projects, settings, activity, statistics and tenant bindings are all listed

### Requirement: The project list, the settings screen and the help overlay need no authority

The project list SHALL remain reachable to every session, because it is where a session starts and where
going back ends, and a reader who may not list projects SHALL be told so by the list's own empty state
rather than by having nowhere to go. The settings screen and the help overlay SHALL likewise need no
authority, because they read nothing from the service.

#### Scenario: A session holding nothing still reaches the project list

- **WHEN** a session's actor holds no scope at all
- **THEN** the project list, the settings screen and the help overlay are reachable

### Requirement: A destructive action in the terminal interface is confirmed against a named subject

The terminal interface SHALL ask for confirmation before performing a destructive action, and the question
SHALL name the subject the action will affect as the reader sees it on screen. A confirmation that cannot name
its subject SHALL NOT be presented.

The question SHALL also state what the action will do beyond its subject where the subject alone cannot say
it, such as a deletion that is permanent or that takes the subtasks with it.

The confirmation SHALL be answered by a key of its own and SHALL NOT be answered by the key that accepts the
interface's other inputs. A key that is neither the agreement nor the cancellation SHALL leave the question
standing rather than answering or dismissing it.

#### Scenario: Deleting a task names the task

- **WHEN** a reader asks to delete the selected task
- **THEN** the confirmation names that task's reference
- **AND** says that it will delete a task

#### Scenario: Deleting a comment names the comment

- **WHEN** a reader asks to delete the selected comment
- **THEN** the confirmation names the comment's author
- **AND** agreeing removes that comment and not the task

#### Scenario: A deletion that reaches further says so

- **WHEN** a reader asks for a deletion that is permanent and takes the subtasks with it
- **THEN** the confirmation says both before it is agreed to

#### Scenario: The key that accepts other inputs does not confirm

- **WHEN** a confirmation is open and the reader presses the accept key
- **THEN** the question is still open
- **AND** nothing has been deleted

#### Scenario: Cancelling a confirmation performs nothing

- **WHEN** a confirmation is open and the reader cancels it
- **THEN** the question is withdrawn
- **AND** nothing has been deleted

### Requirement: The terminal interface gathers structured input in a form

The terminal interface SHALL offer a form of several fields, each answered from the values the operation
accepts, for input the single-line prompt cannot gather. A field SHALL be hidden when another field's answer
makes it irrelevant, and a hidden field SHALL contribute no answer.

Each field SHALL show the values it could hold instead of its current answer, or where its current answer sits
in that list when the list is too long to show.

A form SHALL be moved through and answered with the bindings the interface already defines for moving and
accepting, so that every shipped keybinding scheme drives it, and SHALL name the keys of the loaded scheme
rather than fixed keys.

#### Scenario: A form's own answer takes a field off the screen

- **WHEN** the delete form's subject is changed from the task to a comment
- **THEN** the switches that control how far a task deletion reaches are no longer shown
- **AND** they contribute nothing to the action

#### Scenario: A form is driven under every scheme

- **WHEN** a reader under any of the shipped keybinding schemes opens a form, moves to a field, changes its
  answer and accepts it
- **THEN** the answer reaches the service

#### Scenario: A form names the keys that drive it

- **WHEN** a form is open under a scheme that binds accepting to another key
- **THEN** the form's own instruction names that scheme's key

#### Scenario: A long list of values says where the answer sits

- **WHEN** a field offers more values than its row has room to list
- **THEN** the row states the answer's position in the list instead of listing them

### Requirement: The terminal interface can remove a task

The terminal interface SHALL let a reader delete the selected task, asking first whether the deletion is
permanent and whether it takes the subtasks with it, and SHALL confirm it against the task's own reference.

Once the open task has been deleted the interface SHALL leave its detail view rather than displaying a task the
service no longer holds.

#### Scenario: A deleted task's detail view is left

- **WHEN** the reader deletes the task whose detail view is open
- **THEN** the detail view is no longer open
- **AND** no task is held open

### Requirement: The terminal interface can select a comment in a thread

The terminal interface SHALL let a reader select one comment of the open task's thread, SHALL mark the selected
comment, and SHALL act on that comment when a comment action is performed. The cursor SHALL be offered only
where a thread exists and only in the view that draws one.

#### Scenario: The thread marks the comment the actions will act on

- **WHEN** a task with comments is opened
- **THEN** exactly one comment is marked

#### Scenario: The cursor stays inside the thread

- **WHEN** the reader steps past either end of the thread
- **THEN** the selection stays on the comment at that end

#### Scenario: A task with no comments advertises no cursor

- **WHEN** the open task has no comments
- **THEN** the interface does not advertise a key for stepping the thread

### Requirement: The terminal interface can edit and remove a comment

The terminal interface SHALL let a reader rewrite the selected comment, opening on that comment's own text, and
SHALL let a reader remove it after confirming against a description of that comment. Where there is no comment
to act on the interface SHALL say why rather than opening an input.

#### Scenario: Editing opens on the selected comment's own body

- **WHEN** the reader asks to edit the second comment of a thread
- **THEN** the input opens on that comment's text
- **AND** accepting it rewrites that comment

#### Scenario: A board offers no comment edit

- **WHEN** the reader asks to edit a comment with no task open
- **THEN** no input opens
- **AND** the interface says the task has to be opened

### Requirement: The terminal interface can remove a dependency

The terminal interface SHALL let a reader remove one of the dependencies the open task waits on, choosing from
all of them rather than from a fixed number, and SHALL say so rather than offering a choice when the task waits
on nothing or when no task is open.

#### Scenario: Every dependency is offered

- **WHEN** a task waits on more dependencies than a single-digit choice could number
- **THEN** all of them are offered

#### Scenario: A task that waits on nothing says so

- **WHEN** the reader asks to remove a dependency from a task that waits on nothing
- **THEN** no choice is offered and the interface says the task waits on nothing

### Requirement: The terminal interface offers the tags the tenant has

The terminal interface SHALL offer the tenant's existing tags for a reader to attach to or detach from the
selected task, and SHALL name the tags the task already carries. Where the tenant has no tags the interface
SHALL say so rather than offering an empty choice.

#### Scenario: The picker offers each tag once and names the ones on the task

- **WHEN** a reader opens the tag picker on a task carrying one of the tenant's tags
- **THEN** each of the tenant's tags is offered once
- **AND** the tag the task already carries is named as such

#### Scenario: A tenant with no tags is told so

- **WHEN** a reader opens the tag picker in a tenant with no tags
- **THEN** no picker opens and the interface says no tags exist yet

### Requirement: The terminal interface offers only the actions a reader's authority reaches

The terminal interface SHALL offer an action on the selected task only where the reader holds the authority the
operation needs, in the footer, in the help overlay and on the keystroke alike. A key the reader's scopes do not
reach SHALL change nothing when pressed.

Where one key can act on more than one subject, the interface SHALL offer only the subjects the reader may act
on.

#### Scenario: A refused action is absent from the footer and the overlay

- **WHEN** a reader may not remove a dependency, edit a comment, list tags or delete
- **THEN** none of those keys is advertised in the view's help
- **AND** pressing one changes nothing on screen

#### Scenario: Only the subjects a reader may delete are offered

- **WHEN** a reader may delete a comment but not a task
- **THEN** the delete form offers the comment and not the task

#### Scenario: A reader who may delete nothing is offered no delete

- **WHEN** a reader may delete neither a task nor a comment
- **THEN** pressing the delete key opens nothing

### Requirement: The terminal interface shows the stored history of what the reader is looking at

The terminal interface SHALL offer a view of the durable audit log, scoped to the task the reader has
selected, the project whose screen is open, or the tenant when neither is named.

The view SHALL be distinct from the live event tail. The tail renders the subscription this session holds;
the history renders what the store recorded, including what it recorded before this session opened. Neither
SHALL draw the other's records.

The view SHALL name the subject whose history it is drawing, SHALL resolve the actor of each entry to the
handle a reader recognises where that lookup succeeds, and SHALL say when the log runs past the page it
drew, naming the command that reads the rest.

A reader who may not read the audit log SHALL NOT be offered the view, SHALL NOT be told about its key, and
pressing that key SHALL issue no read.

#### Scenario: The history of the selected task

- **WHEN** a reader with audit authority asks for the history from a board with a task selected
- **THEN** the log is read for that task
- **AND** the view names the task whose history it is

#### Scenario: The live tail and the stored log stay apart

- **WHEN** a session has seen a live event and the store holds an older entry
- **THEN** the history view draws the stored entry and not the live event
- **AND** the activity view draws the live event and not the stored entry

#### Scenario: A subject with no history says so

- **WHEN** the log holds nothing for the subject
- **THEN** the view says so in words and names the subject it found nothing for

#### Scenario: A reader without audit authority reaches nothing

- **WHEN** a reader who may not read the audit log presses the history key
- **THEN** no view opens
- **AND** no audit read is issued

### Requirement: The terminal interface records an artifact on a task

The terminal interface SHALL record structured output on the selected task, gathering a name and a kind. The
kinds offered SHALL be exactly the kinds the service accepts, so the interface can neither omit one nor
offer one that would be refused.

The interface SHALL state, where the artifact is gathered, which parts of an artifact it does not record.

An artifact with no name SHALL NOT be recorded, so a stray keystroke never attaches an empty artifact.

A reader who may not write artifacts SHALL NOT be told about the key, and pressing it SHALL record nothing.

#### Scenario: A named artifact of a chosen kind

- **WHEN** a reader names an artifact and chooses a kind
- **THEN** the artifact is recorded on the selected task under that name and that kind

#### Scenario: Every kind the service accepts is reachable

- **WHEN** the kind is chosen
- **THEN** every artifact kind the service declares is among the alternatives

#### Scenario: What the terminal does not record is said out loud

- **WHEN** the artifact form is open
- **THEN** it says that the payload, the content type and the inline blob are recorded elsewhere

### Requirement: The terminal interface restores a deleted task from the board that reveals it

The board's filter SHALL be the way deleted tasks are seen, through the same expression the command line
takes, and a deleted card SHALL carry a marker distinguishing it from live work, explained in the legend.

The interface SHALL restore the selected task where that task is deleted. Where it is not, the interface
SHALL refuse without calling the service and SHALL name the filter that reveals the deleted tasks.

The footer of a deleted card SHALL offer the restore and SHALL NOT offer the actions the service refuses on
a deleted task.

A reader who may not restore SHALL NOT be offered the key, and pressing it SHALL issue no call.

#### Scenario: A deleted card is told apart from a live one

- **WHEN** the board's filter includes deleted tasks
- **THEN** a deleted card carries the deleted marker and a live card does not

#### Scenario: Restoring the selected deleted task

- **WHEN** a reader restores a deleted card
- **THEN** that task is restored and the status bar names it

#### Scenario: A task that was never deleted is not restored

- **WHEN** a reader presses the restore key on a live task
- **THEN** nothing is sent to the service
- **AND** the refusal names the filter that reveals the deleted tasks

### Requirement: The terminal interface assigns a task from the tenant's directory

The terminal interface SHALL offer the tenant's actors when a task is assigned, named by the handle a reader
recognises rather than by the identifier the service stores, and SHALL resolve the chosen name back to that
identifier.

The offer SHALL include a way to leave the task assigned to nobody, so clearing an assignment is a choice on
the same list.

The picker SHALL open on whoever holds the task now. An assignee the directory no longer holds SHALL open on
nobody rather than on another actor.

A reader who may not read the directory SHALL NOT be offered the picker, whatever else they may change about
a task, and a tenant with no actors SHALL say so rather than opening a picker with nothing to pick.

#### Scenario: Assigning by handle

- **WHEN** a reader picks an actor by handle
- **THEN** the task is assigned to that actor's identifier

#### Scenario: Clearing an assignment

- **WHEN** a reader chooses nobody
- **THEN** the task is left with no assignee

#### Scenario: A reader who cannot read the directory is offered no picker

- **WHEN** a reader may change a task and may not list actors
- **THEN** the assignee key is not offered

### Requirement: Display preferences in the terminal interface

The terminal interface SHALL offer a settings view carrying the display preferences a reader owns: the
keybinding scheme, the time format, the timezone and the colour mode. Each SHALL name the configuration
key it is written to, and SHALL show what choosing a value would mean, rendered by the renderer that will
render it, so an example can never claim a layout or a zone the interface does not produce.

A value the build does not ship, arriving from a configuration file written elsewhere, SHALL remain on
offer rather than being dropped, so stepping through the values cannot silently discard it.

#### Scenario: Every preference names its key

- **WHEN** the settings view is open
- **THEN** each preference row names the configuration key its value is written to

#### Scenario: An example is rendered in the chosen zone

- **WHEN** the timezone row is stepped from one zone to another
- **THEN** the example beside it renders the same instant in the newly chosen zone

#### Scenario: An example is rendered in the chosen layout

- **WHEN** the time format row is stepped from one layout to another
- **THEN** the example beside it renders in the newly chosen layout

#### Scenario: Stepping wraps rather than stopping

- **WHEN** a preference is stepped past either end of its values
- **THEN** it continues from the other end

#### Scenario: A configured value this build does not ship is kept

- **WHEN** a preference holds a value that is not one this build offers
- **THEN** that value is still among the values the row steps through

### Requirement: A display preference chosen in the terminal interface persists

A preference changed in the settings view SHALL take effect in the frame it is read in and SHALL be
written to the reader's configuration file in the same act, with no separate save. The interface SHALL
state where the change was written.

The write SHALL edit the configuration file in place, carrying forward the keys it already held and
writing only the keys that changed, so choosing a display preference never pins a value another layer was
supplying.

A session with no configuration file to write SHALL say that its choices last only for the session,
rather than offering a choice it cannot keep. A write that fails SHALL be reported, naming the failure.

#### Scenario: A change is written down at once

- **WHEN** a preference is stepped to a new value
- **THEN** the interface adopts it, writes it to the configuration file, and says which file it wrote

#### Scenario: The choice survives a restart

- **WHEN** the interface is closed and opened again
- **THEN** the preference chosen in the previous run is the one in force

#### Scenario: Writing one preference pins nothing else

- **WHEN** a display preference is written to a configuration file
- **THEN** no key that the file did not already carry, and that the change did not alter, is written to it

#### Scenario: A session that cannot write says so

- **WHEN** the session was opened with no way to write a configuration file
- **THEN** the screen states that the choices last until the interface is quit, and a change says the same

#### Scenario: A failed write is reported

- **WHEN** writing the configuration file fails
- **THEN** the interface says the change was not saved and names the failure

#### Scenario: A value the build cannot render is refused

- **WHEN** a preference would be set to a value this build cannot resolve
- **THEN** the value is not adopted and the refusal names it

### Requirement: The settings view states which layer supplies each preference

Where a preference's value arrives from a configuration layer above the file, the settings view SHALL say
so on that preference's own row, and SHALL state that the layer still decides after a restart. A value
arriving from the file or from the built-in defaults SHALL carry no such warning.

#### Scenario: An environment variable is announced

- **WHEN** a preference's value is supplied by an environment variable
- **THEN** that preference's row says the environment layer supplies it and still wins after a restart

#### Scenario: A file value carries no warning

- **WHEN** a preference's value comes from the configuration file or from the defaults
- **THEN** that preference's row carries no warning about another layer

### Requirement: The settings view states what the session is connected to

The settings view SHALL state the target this run resolved, the tenant, the actor, the build and the
configuration file. The target SHALL be stated with its secrets redacted, so a connection string carrying
a password is never drawn on a screen. A fact the session was not told SHALL say so rather than rendering
blank, which reads as a value that failed to load.

The tenant and the actor SHALL be the ones in force, so a session that has switched tenant reports the
tenant it switched to.

The view SHALL name the command that answers the same question across every configuration key, and SHALL
NOT offer to change the target, the credentials or any other deployment setting: a session is already
connected through the target it resolved and could not act on a new one.

#### Scenario: The connection is named

- **WHEN** the settings view is open
- **THEN** it states the target, the tenant, the actor, the build and the configuration file

#### Scenario: A secret in the target is redacted

- **WHEN** the resolved target is a connection string carrying a password
- **THEN** the password does not appear on the screen

#### Scenario: The tenant follows a switch

- **WHEN** the session has switched to another tenant
- **THEN** the settings view names the tenant the session switched to

#### Scenario: A fact the session was not told

- **WHEN** a fact was never supplied to the session
- **THEN** the row says so rather than rendering empty

#### Scenario: The rest of the configuration is pointed at, not duplicated

- **WHEN** the settings view is open
- **THEN** it names `tix config show --sources` and offers no control over the target or the credentials

### Requirement: The whole settings view is reachable on a short terminal

The settings view SHALL be navigable to its last line on a terminal too short to show it at once, and
SHALL never draw more lines than the frame has. The keys that step a preference's values SHALL be
described as doing that, rather than by the meaning they carry on the board.

#### Scenario: The bottom of the screen is reachable

- **WHEN** the terminal is too short to show the whole settings view and the reader walks down
- **THEN** the last line of the view comes into the window

#### Scenario: The body never overflows the frame

- **WHEN** the settings view is drawn into a frame shorter than the view
- **THEN** it draws no more lines than the frame holds and says how many are off screen

#### Scenario: The footer describes the settings view

- **WHEN** the settings view is open
- **THEN** its footer describes the value keys as stepping a setting's values, not as moving between columns

### Requirement: The terminal interface states how a project is configured

The terminal interface SHALL offer a screen that states, for one project, the project's own attributes, the
workflow its tasks move through and the custom field definitions those tasks carry.

The screen SHALL read the project it shows rather than reusing a row from a listing, so that what it states is
true after an action it performed.

The workflow SHALL be shown as its state machine: the initial state, every state with the category it reports
under and whether it is terminal, every permitted edge with what that edge requires, and the lease a claim
takes by default.

An attribute that is not set SHALL be shown as unset rather than as an empty row, so a blank cannot be
mistaken for a row that failed to draw.

A part of the screen the reader's authority does not reach SHALL say why it is not shown, and SHALL NOT cost
the reader the rest of the screen.

#### Scenario: The screen names the project it read

- **WHEN** a reader opens the project screen
- **THEN** it states that project's key, name, description, colour, icon and whether it is archived
- **AND** the project was read rather than taken from the listing

#### Scenario: The workflow is shown as a state machine

- **WHEN** the project screen is open
- **THEN** it names the workflow, its initial state, each of its states and each permitted edge
- **AND** marks a state a task stops in as terminal

#### Scenario: An archived project says when it was archived

- **WHEN** the project screen shows an archived project
- **THEN** it states that it is archived and when

#### Scenario: A refused workflow costs nothing else

- **WHEN** the reader may read the project but not its workflow
- **THEN** the screen says why the workflow is not shown
- **AND** still states the project's own attributes

#### Scenario: The screen opens on the project the reader chose

- **WHEN** a reader presses the key from the project listing
- **THEN** the screen opens on the row under the cursor
- **AND** pressing it from a board opens the project the board has open

### Requirement: A workflow is read in the terminal and changed elsewhere

The terminal interface SHALL NOT offer an editor for a workflow definition, and the screen that renders a
workflow SHALL name where a workflow is changed instead.

#### Scenario: The screen points at the command that edits a workflow

- **WHEN** the project screen renders a workflow
- **THEN** it names the command that changes one

### Requirement: The terminal interface edits every attribute a project accepts

The terminal interface SHALL offer, from the project screen, a change to each attribute the project update
accepts. An attribute whose values are a fixed set SHALL be answered from that set; an attribute that is free
text SHALL be answered by the single-line prompt, seeded with the value it would replace.

An attribute with no alternative to offer SHALL NOT be offered, and a change SHALL send only the attribute it
was asked for.

#### Scenario: A colour is chosen from the palette

- **WHEN** a reader chooses the colour attribute and steps its value
- **THEN** the value on screen is the value sent
- **AND** no other attribute is sent with it

#### Scenario: A name is typed into the prompt it opens

- **WHEN** a reader chooses the name attribute
- **THEN** the prompt opens seeded with the project's current name
- **AND** accepting it sends only the name

#### Scenario: One workflow is no choice

- **WHEN** the tenant has a single workflow
- **THEN** the project screen offers no workflow to move to

### Requirement: Archiving and deleting a project are confirmed against the named project

The terminal interface SHALL ask, before hiding or destroying a project, which of the two is meant, and SHALL
then confirm it against the project named as the reader sees it.

The confirmation for a deletion SHALL state how far the deletion reaches. The confirmation for an archive
SHALL NOT claim a deletion's reach.

An archive SHALL NOT be offered for a project that is already archived, because the service refuses it.

Once a project is deleted the interface SHALL leave the screen that showed it rather than reading a project
the service has removed.

#### Scenario: An archive names the project and nothing more

- **WHEN** a reader asks to archive the open project
- **THEN** the confirmation names that project and says it will archive it
- **AND** agreeing archives it and deletes nothing

#### Scenario: A deletion states its reach

- **WHEN** a reader asks to delete the open project
- **THEN** the confirmation names the project and states what goes with it
- **AND** agreeing deletes it and returns the reader to the project listing

#### Scenario: An archived project is not offered archiving

- **WHEN** the project screen shows an archived project
- **THEN** archiving is not among the actions offered

#### Scenario: Cancelling leaves the project alone

- **WHEN** a confirmation about a project is cancelled
- **THEN** the project is neither archived nor deleted

### Requirement: The terminal interface defines and removes a project's custom fields

The terminal interface SHALL define a custom field from the project screen, gathering its key and label as
text and the answers drawn from fixed sets in a form.

A redefinition SHALL be seeded from the definition it replaces, and SHALL carry through every part of that
definition the form did not ask about, because the operation replaces the whole definition.

A field type whose values the form cannot gather SHALL be offered only to a definition that already holds it.

Removing a definition SHALL be confirmed against the field named as the screen shows it.

A picker SHALL NOT be offered where there is nothing to pick.

#### Scenario: A new field is named and then shaped

- **WHEN** a reader defines a new custom field
- **THEN** the key and the label are taken as text
- **AND** the type and whether it is required are answered from their own lists
- **AND** the definition sent is one the operation accepts

#### Scenario: A redefinition starts from the field that was picked

- **WHEN** a reader picks the second of two definitions to redefine
- **THEN** the form opens on that definition's own type and requirement
- **AND** the parts the form did not ask about are sent unchanged

#### Scenario: An enumerated type is not offered to a field that cannot carry it

- **WHEN** a reader defines a new field
- **THEN** the type list omits the type whose values the form cannot gather
- **AND** a definition that already holds that type keeps it

#### Scenario: Removing a definition names it

- **WHEN** a reader asks to remove a custom field definition
- **THEN** the confirmation names that field
- **AND** agreeing removes that definition and no other

#### Scenario: A project with no definitions offers no picker

- **WHEN** the project defines no custom fields
- **THEN** the picker is not advertised
- **AND** the key that defines a new one still is

### Requirement: Every project affordance is gated by the authority it needs

The terminal interface SHALL offer each action on the project screen only to a reader who holds the authority
the operation it calls requires, and SHALL refuse the keystroke on the same question the footer and the help
overlay are filtered by.

An action the screen borrows a key for SHALL be described by what it does on that screen.

#### Scenario: A reader who may only read is offered nothing to change

- **WHEN** a reader who may read a project but not write one opens the screen
- **THEN** no action key is advertised
- **AND** pressing one opens no input and reaches no service call

#### Scenario: The overlay narrows to the reader

- **WHEN** a reader may edit a project and nothing else on the screen
- **THEN** the overlay names the edit and none of the others

#### Scenario: A borrowed key is described by what it does here

- **WHEN** the overlay describes the project screen
- **THEN** the edit key is described as editing a project

### Requirement: The selected row pulses

The terminal interface SHALL animate the selected row by alternating the emphasis it is drawn with, rather
than by using the terminal's blink attribute, so that every terminal able to draw colour draws the same
animation. No other element of the interface SHALL animate.

The selection SHALL remain unambiguous at every point in the cycle: the selection marker and the
selection's colour SHALL be drawn in every frame, and only the emphasis SHALL vary. A reader looking at
the screen at any moment SHALL be able to tell which row is selected.

#### Scenario: The selected row changes emphasis

- **WHEN** consecutive phases of the pulse are drawn
- **THEN** the selected row is rendered differently in each

#### Scenario: The marker survives both phases

- **WHEN** the selected row is drawn at either phase of the pulse
- **THEN** it carries the selection marker and is rendered differently from an unselected row

#### Scenario: Nothing else moves

- **WHEN** the interface is idle at the keyboard but connected
- **THEN** only the selected row's emphasis changes, and no other element animates

### Requirement: An idle session animates nothing

The interface SHALL stop the pulse after a bounded period with no input, and SHALL produce no further
frames until input arrives. It SHALL start the pulse again on the next keystroke.

A stopped pulse SHALL rest on the emphasised phase, which is the same rendering a session with motion
turned off draws, so a session that has gone quiet cannot be mistaken for one that has stopped responding.

#### Scenario: The frames stop

- **WHEN** the idle period passes with no keystroke
- **THEN** the interface produces no further frames until a key arrives

#### Scenario: A phase arriving after the pause changes nothing

- **WHEN** a phase of the pulse is delivered after the idle pause has taken effect
- **THEN** the frame is unchanged and no further phase is scheduled

#### Scenario: A keystroke starts it again

- **WHEN** a key arrives after the idle pause has stopped the pulse
- **THEN** the pulse resumes

### Requirement: A terminal that cannot show the pulse gets a static selection

The interface SHALL animate only when it is drawing in colour. A session whose colour is suppressed, by
`NO_COLOR`, by `TIX_NO_COLOR`, by `output.color = never` or by a destination that is not a terminal, SHALL
draw a static selection and SHALL produce no animation frames at all.

#### Scenario: A colourless session never pulses

- **WHEN** a session is drawing without colour
- **THEN** no phase of the pulse is ever scheduled and the selected row is drawn the same in every frame

### Requirement: Motion is a display preference on the settings screen

The settings screen SHALL offer motion as a display preference alongside the keybinding scheme, the time
format, the timezone and the colour mode. It SHALL be on unless it is turned off, including in a session
given no preferences at all.

The row SHALL name the configuration key it is written to, SHALL say what the chosen value means on this
terminal, SHALL apply the change to the frame it is read in, SHALL write the change down at once, and SHALL
report the configuration layer supplying the value when a layer above the file supplies it. A session with
nowhere to write SHALL keep the row usable and say the choice lasts only for the session.

#### Scenario: Motion is on by default

- **WHEN** a session is opened with nothing configured
- **THEN** the selected row pulses

#### Scenario: The row is written down when it is stepped

- **WHEN** the motion row is stepped to another value
- **THEN** the interface adopts it for the open frame and writes it to the configuration file

#### Scenario: Turning motion off stops the animation

- **WHEN** the motion row is stepped to off
- **THEN** no further phase of the pulse is scheduled

#### Scenario: The row says what it means on this terminal

- **WHEN** the motion row is read on a session that is drawing without colour
- **THEN** it says that nothing will pulse, rather than that motion is running

### Requirement: The terminal interface states what the tenant is

The terminal interface SHALL offer a screen that states, for the tenant the session is pinned to, the
tenant's own attributes, the tenants this session can see, the hostnames that resolve to it and the actors
who belong to it.

The screen SHALL read what it states rather than rendering it from what the session already held, so that
what it says is true after an action it performed.

A section the reader's authority does not reach SHALL say why it is not shown, and SHALL NOT cost the reader
the rest of the screen.

A member SHALL be named by the handle a reader recognises wherever the handle can be resolved, and by the
identifier the service stores otherwise, so that a row is never blank.

#### Scenario: The screen names the tenant it read

- **WHEN** a reader opens the tenant screen
- **THEN** it states that tenant's key, name and theme
- **AND** the tenant was read rather than taken from the session's configuration

#### Scenario: The domains and the members are listed

- **WHEN** the tenant screen is open
- **THEN** it lists each hostname that resolves to the tenant
- **AND** lists each member with the role that member holds

#### Scenario: A refused section costs nothing else

- **WHEN** the reader may reach the screen but not read its domains
- **THEN** the domains section says why it is not shown
- **AND** the screen still states the tenant's own attributes

#### Scenario: The switch the screen already offered still works

- **WHEN** a reader with a way to dial another tenant opens the screen
- **THEN** it still takes a tenant key and switches the session to it
- **AND** a session with no way to dial still says so and names the command that can

### Requirement: The tenant screen selects the row an action acts on

The terminal interface SHALL put one cursor over the tenant screen's domains and members together, and each
selected row SHALL carry which of the two it is.

A screen holding no domain and no member SHALL have no selection, and the removal key SHALL say there is
nothing to remove rather than opening a question over nothing.

#### Scenario: The cursor moves through both listings

- **WHEN** a reader moves the selection down past the last domain
- **THEN** the selection lands on the first member

#### Scenario: A screen with nothing to remove refuses the key

- **WHEN** the tenant has no domains and no members
- **THEN** pressing the removal key opens no confirmation
- **AND** says there is nothing on this screen to remove

### Requirement: The terminal interface edits the tenant's attributes

The terminal interface SHALL offer, from the tenant screen, an edit of the tenant's name and of the theme it
presents itself with.

The theme SHALL be answered from the palettes the build carries, together with a value standing for having
none, so that a theme can be cleared by choosing rather than by typing nothing.

The name SHALL be gathered by the single-line input, seeded with the name it would replace.

#### Scenario: The theme is chosen from what the build has

- **WHEN** a reader opens the tenant edit and selects the theme attribute
- **THEN** the values offered are the palettes the build carries and a value meaning none

#### Scenario: The name edit starts from the current name

- **WHEN** a reader opens the tenant edit and selects the name attribute
- **THEN** the single-line input opens carrying the tenant's current name

### Requirement: The terminal interface adds a domain and a member

The terminal interface SHALL offer, from the tenant screen, one key that asks whether a domain or a member is
being added.

A domain SHALL gather its hostname through the single-line input, and SHALL be added carrying no certificate
of its own, because a certificate is a pair of paths on the server that no control here can gather.

A member SHALL gather both of its answers from fixed lists: the actor from the tenant's own directory, read at
the keystroke rather than held for the session, and the role from the roles the service accepts.

A tenant whose directory holds nobody SHALL say so rather than opening a picker with nothing in it.

#### Scenario: A domain is added by hostname

- **WHEN** a reader chooses to add a domain and types a hostname
- **THEN** the domain is added with the hostname that was typed
- **AND** it carries no certificate of its own

#### Scenario: A member is picked from the directory

- **WHEN** a reader chooses to add a member
- **THEN** the actors offered are the ones the directory was read for at that keystroke
- **AND** the role offered is one the service accepts

#### Scenario: An empty directory opens no picker

- **WHEN** the tenant's directory holds nobody
- **THEN** no member form opens
- **AND** the screen says the tenant has nobody to add

### Requirement: Removing a domain or a member names its subject

The terminal interface SHALL hold a removal of a domain or of a membership behind a confirmation that names
the hostname or the handle it will remove.

The confirmation SHALL be answered by the agreement key rather than by the key every other input is accepted
with.

#### Scenario: The question names the hostname

- **WHEN** a reader presses the removal key on a selected domain
- **THEN** the confirmation names that hostname

#### Scenario: The question names the member

- **WHEN** a reader presses the removal key on a selected member
- **THEN** the confirmation names that member's handle

#### Scenario: Declining removes nothing

- **WHEN** a reader dismisses the confirmation
- **THEN** no removal is sent to the service

### Requirement: The tenant screen offers only what the reader's authority reaches

The terminal interface SHALL gate each of the tenant screen's actions on the operation it calls, in the
footer, in the help overlay and at the keystroke, so that none of the three can offer what the other two
refuse.

#### Scenario: A reader without tenant administration is offered none of the actions

- **WHEN** a reader who may not administer the tenant opens the screen
- **THEN** the footer offers none of the edit, add or remove keys
- **AND** pressing one of them opens nothing and sends nothing
