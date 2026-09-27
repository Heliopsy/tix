## ADDED Requirements

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
