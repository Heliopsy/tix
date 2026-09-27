## ADDED Requirements

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
