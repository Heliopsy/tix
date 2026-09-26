## ADDED Requirements

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
