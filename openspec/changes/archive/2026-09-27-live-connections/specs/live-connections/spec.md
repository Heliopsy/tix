## ADDED Requirements

### Requirement: Listing the connections a server holds

An administrator SHALL be able to list the live connections of their own tenant, across both the event
stream and SSH. Each SHALL report an identifier, the surface it arrived on, the actor behind it, where it
came from and when it opened. An SSH connection SHALL also report the key fingerprint that opened it.

#### Scenario: Both surfaces are listed

- **WHEN** a tenant holds a live event-stream connection and a live SSH session and an administrator lists
  connections
- **THEN** both are returned, each naming its surface, its actor and when it opened

#### Scenario: A closed connection is gone

- **WHEN** a connection closes and connections are listed again
- **THEN** it is absent, without having to be removed by hand

#### Scenario: Nothing live

- **WHEN** no connection is open and connections are listed
- **THEN** an empty list is returned rather than an error

### Requirement: Counting connections

Listing SHALL report the number of live connections for the caller's tenant, broken down by surface, and the
number the process is holding in total. The process total SHALL carry no breakdown by tenant, actor or
address.

#### Scenario: Tenant counts and a process total

- **WHEN** an administrator of one tenant lists connections while another tenant also has connections open
- **THEN** the per-surface counts cover only the caller's tenant, and the process total covers every
  connection without revealing anything about who holds them

### Requirement: Ending a connection

An administrator SHALL be able to end one live connection of their own tenant by identifier. The connection
SHALL be closed, and the actor holding it SHALL be able to reconnect unless separately prevented.

#### Scenario: A connection is ended

- **WHEN** an administrator ends a live connection by identifier
- **THEN** that connection closes, and no other connection is affected

#### Scenario: Ending is recorded before it happens

- **WHEN** a connection is ended
- **THEN** an audit entry naming the actor ended, the surface and the caller is committed before the
  connection is closed

#### Scenario: An identifier that is not live

- **WHEN** an identifier is given that names no live connection
- **THEN** the request reports that nothing matched, and nothing else is closed

#### Scenario: Ending is not revocation

- **WHEN** a connection is ended and its holder reconnects with credentials that are still valid
- **THEN** the new connection is accepted

### Requirement: Connections are tenant-scoped

A connection SHALL NOT be listed, counted in a tenant breakdown, addressed or ended from any tenant but its
own, through any surface.

#### Scenario: Another tenant's connection is invisible

- **WHEN** an administrator of one tenant lists connections while another tenant holds several
- **THEN** none of the other tenant's connections appear, and none of their identifiers, actors or addresses
  are disclosed

#### Scenario: Another tenant's connection cannot be ended

- **WHEN** an administrator names the identifier of a connection belonging to another tenant
- **THEN** the request reports that nothing matched, that connection stays open, and the response does not
  reveal that the identifier exists

### Requirement: The view is one server's

A response SHALL identify the server that answered, and SHALL NOT imply that it covers connections held by
any other process.

#### Scenario: Several servers on one database

- **WHEN** more than one server runs against the same database and connections are listed from one of them
- **THEN** the response covers that server's connections and says which server answered

### Requirement: A draining shutdown tells a session why it ended

When a listener is draining and a session is still open at the deadline, that session SHALL be told why it
is being closed before the connection goes.

#### Scenario: A session outlives the shutdown timeout

- **WHEN** a shutdown drains the SSH listener and a session is still open when the timeout expires
- **THEN** the session is told the server is shutting down, and is then closed

#### Scenario: A session that finishes in time

- **WHEN** a session ends before the shutdown timeout expires
- **THEN** it is not interrupted and receives no such notice
