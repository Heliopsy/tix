## ADDED Requirements

### Requirement: The terminal interface shows the server's status and live connections

The terminal interface SHALL offer a server view, to a reader who may read the status report, that states
the installation, the answering server, every registered server with whether it still heartbeats, the
tenant's work counts and its second factor state, and the tenant's live connections on the answering
server.

#### Scenario: The status and the connections are read when the view opens

- **WHEN** a tenant admin opens the server view
- **THEN** it states the installation's version and schema and lists each registered server
- **AND** lists each live connection with its surface, handle and address

#### Scenario: A refused section costs nothing else

- **WHEN** the connections cannot be read
- **THEN** the connections section says why it is not shown
- **AND** the status sections are still drawn

### Requirement: Ending a live connection names it and is confirmed

The terminal interface SHALL put a cursor over the live connections and SHALL hold ending the selected one
behind a confirmation naming its surface, handle and address, which says the holder may reconnect.

A view with no connection SHALL refuse the key rather than open a question over nothing.

#### Scenario: The question names the connection

- **WHEN** a reader presses the removal key on a selected connection
- **THEN** the confirmation names the connection's surface, handle and address

#### Scenario: Agreeing ends it

- **WHEN** the reader agrees
- **THEN** the connection is ended through the service and the view is read again

### Requirement: The terminal interface offers a global admin view only to a global admin

The terminal interface SHALL offer a global admin view only to a session whose actor the service reports
as a global admin when the session opens, resolved from the same report the web sidebar uses.

A reader who is not a global admin SHALL NOT be offered the view's key, and pressing it SHALL enter
nothing.

#### Scenario: A global admin is offered the view

- **WHEN** a session opens for an actor the service reports as a global admin
- **THEN** the global admin view is offered

#### Scenario: A tenant admin is not

- **WHEN** a session opens for a tenant admin who is not a global admin
- **THEN** the global admin view is not offered and its key enters nothing

### Requirement: The global admin view lists, creates and deletes tenants

The global admin view SHALL list every tenant and SHALL create one from a key, a name and an optional list
of first admins' emails, showing any password the service generated once.

Deleting a tenant SHALL be confirmed against its key, and the tenant the session is pinned to SHALL NOT be
deleted from here.

#### Scenario: Generated passwords are shown once

- **WHEN** a tenant is created naming a first admin with no account
- **THEN** the view shows that admin's generated password
- **AND** it is gone when the view is read again

#### Scenario: The session's own tenant is refused

- **WHEN** a reader presses the removal key on the tenant the session is pinned to
- **THEN** no confirmation opens and the view says why

### Requirement: The global admin view manages any tenant's users

The global admin view SHALL open the users of the selected tenant, with each user's role and whether they
are disabled, and SHALL create a user, add an existing account by email, edit a user's name, role and
disabled state, and delete a user after a confirmation naming them.

#### Scenario: An existing account is added

- **WHEN** a reader adds an existing account to a tenant by email with a role
- **THEN** the account is added to that tenant with that role through the service

#### Scenario: A new user's password is never drawn

- **WHEN** a reader types a password into the new user form
- **THEN** the frame shows it masked

### Requirement: The global admin view manages global admins and reads any account's log

The global admin view SHALL list the global admins, grant global admin by email and revoke it after a
confirmation naming the address, and SHALL read the log of any account by email.

#### Scenario: An account log is read by email

- **WHEN** a reader asks for the log of an account by its email
- **THEN** the view lists that account's log entries, newest last

#### Scenario: Back leaves a mode before the view

- **WHEN** a reader presses Back on a tenant's users or an account log
- **THEN** the view returns to the tenant listing rather than leaving
