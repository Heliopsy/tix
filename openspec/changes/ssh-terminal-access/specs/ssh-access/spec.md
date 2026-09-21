## ADDED Requirements

### Requirement: SSH listener for the terminal interface

The ssh command SHALL serve the terminal interface over SSH from the tix process itself, without requiring a
system SSH daemon, a system user account or a login shell. It SHALL bind a loopback address and port 2222 by
default.

#### Scenario: A client lands on a board

- **WHEN** an SSH client connects with a public key and requests a pseudo-terminal
- **THEN** the terminal interface is drawn over the session and responds to the keys it responds to locally

#### Scenario: No pseudo-terminal

- **WHEN** an SSH client connects without requesting a pseudo-terminal
- **THEN** the session is refused with a message naming the missing terminal

### Requirement: The key fingerprint is the identity

The listener SHALL accept any public key a client proves, without the key having been registered beforehand,
and SHALL resolve that key's fingerprint to the actor the session runs as. No password and no signup SHALL be
required.

#### Scenario: An unknown key is accepted

- **WHEN** a client connects with a public key the listener has never seen
- **THEN** the connection is authenticated and a session begins

#### Scenario: A password is not a credential

- **WHEN** a client attempts to authenticate without a public key
- **THEN** the connection is refused

### Requirement: An ephemeral tenant per fingerprint

The listener SHALL give each distinct key fingerprint its own tenant, seeded on first connection with demo
content through the same import path a hand-written snapshot uses. A fingerprint that has connected before
SHALL be returned to its existing tenant rather than given a new one.

#### Scenario: First connection

- **WHEN** a fingerprint connects for the first time
- **THEN** a tenant is created for it and seeded with a project and tasks, and the session opens on that board

#### Scenario: Return connection

- **WHEN** a fingerprint that has connected before connects again
- **THEN** the session opens on the same tenant, including every change made in earlier sessions, and nothing is
  reseeded

#### Scenario: Two fingerprints are two tenants

- **WHEN** two different keys connect
- **THEN** each session runs in its own tenant, and neither can read, list or address anything belonging to the
  other

### Requirement: Session isolation under concurrency

Each connection SHALL build its own actor, context and model. No state that carries a tenant SHALL be shared
between concurrent sessions.

#### Scenario: Concurrent sessions

- **WHEN** many sessions under different fingerprints run at once and each writes a task only it could write
- **THEN** every session's listing contains only its own tasks

#### Scenario: Addressing another tenant's row

- **WHEN** a session names a task identifier belonging to another tenant
- **THEN** the operation reports not found rather than returning the row

### Requirement: Visitor authority is an explicit scope grant

A visitor SHALL hold an explicit set of scopes rather than a named role. The grant SHALL include the scopes
needed to work tasks, projects and workflows, and SHALL exclude the scopes that administer the tenant, mint
credentials, create users, register webhooks, run external sync or bulk import.

#### Scenario: Working the board

- **WHEN** a visitor creates a task, transitions it, claims it, or edits a project or workflow
- **THEN** the operation is permitted

#### Scenario: Reaching outside the sandbox

- **WHEN** a visitor attempts to administer the tenant, create a token, create a user, register a webhook or
  import a snapshot
- **THEN** the operation is refused

### Requirement: Expiry slides from the last connection

Each tenant the listener owns SHALL carry a last-seen time, updated on every successful connection. A
background reaper SHALL delete a tenant not seen within a configurable time to live, together with every row
belonging to it. It SHALL NOT delete a tenant the listener does not own.

#### Scenario: A returning visitor keeps their board

- **WHEN** a visitor connects, and connects again before the time to live elapses
- **THEN** the tenant survives, measured from the later connection, not from its creation

#### Scenario: An abandoned sandbox is deleted

- **WHEN** a tenant has not been connected to for the time to live
- **THEN** it is deleted and no task, project, actor or workflow belonging to it remains

#### Scenario: A reaped fingerprint returns

- **WHEN** a fingerprint whose tenant was deleted connects again
- **THEN** a fresh seeded tenant is created for it and the connection succeeds

#### Scenario: Other tenants are untouched

- **WHEN** the reaper runs against a database that also holds tenants the listener did not create
- **THEN** those tenants are left alone regardless of age

### Requirement: Caps on tenants and tasks

The listener SHALL refuse to create a tenant once a configurable number are live, and SHALL refuse to create a
task once a tenant holds a configurable number. A refusal SHALL name the limit. An existing tenant SHALL NEVER
be deleted to admit a new fingerprint.

#### Scenario: The listener is full

- **WHEN** a new fingerprint connects while the tenant cap is reached
- **THEN** the connection is refused with a message explaining that the demo is full and that sandboxes expire
  on their own

#### Scenario: An existing visitor at the cap

- **WHEN** a fingerprint that already owns a tenant connects while the cap is reached
- **THEN** the session opens normally on its own tenant

#### Scenario: A full sandbox

- **WHEN** a visitor creates a task in a tenant already holding the task cap
- **THEN** the creation is refused with a message naming the limit, and existing tasks are unaffected

### Requirement: Connection rate limiting

The listener SHALL limit connection attempts per source address, allowing a configurable burst and refilling at
a configurable rate. The memory it uses for this SHALL be bounded.

#### Scenario: A source exceeds its allowance

- **WHEN** one source address exceeds its burst within the refill window
- **THEN** further connections from that address are refused until the allowance refills

#### Scenario: Other sources are unaffected

- **WHEN** one source address is being refused
- **THEN** a different source address connects normally

### Requirement: Persisted host key

The listener SHALL load a host key from a configurable path, generating one on first run with permissions
readable only by its owner. It SHALL refuse to start on a key file readable by anyone else.

#### Scenario: First run

- **WHEN** the listener starts and no host key exists at the configured path
- **THEN** a key is generated, written at mode 0600, and reused unchanged on every later start

#### Scenario: A loose host key

- **WHEN** the host key file is readable by anyone but its owner
- **THEN** the listener refuses to start and names the file and its mode

### Requirement: Bind and target safety

The listener SHALL refuse to bind a non-loopback address unless an explicit opt-out is supplied, and SHALL
refuse to run against the zero-configuration database. It SHALL require a local database target.

#### Scenario: Public bind without the opt-out

- **WHEN** a non-loopback listen address is configured without the explicit opt-out
- **THEN** the listener refuses to start and explains that the choice must be explicit

#### Scenario: No database named

- **WHEN** the ssh command is run without a database or server being named anywhere
- **THEN** it refuses to start, explaining that the zero-configuration store holds real work and that this
  listener needs a database of its own

### Requirement: The visitor is told the board is temporary

The interface SHALL tell a visitor, unobtrusively, that the board is a demo sandbox and roughly how long it
survives unvisited.

#### Scenario: A visitor reads the notice

- **WHEN** a session opens on a freshly seeded tenant
- **THEN** the board carries a row saying that it is a demo sandbox owned by the visitor's key and naming the
  time to live

#### Scenario: The session ends

- **WHEN** a visitor leaves the interface
- **THEN** a single line repeats that the sandbox is keyed to their SSH key and how long it survives unvisited

### Requirement: Colour follows what the client said

The listener SHALL decide whether a session is drawn in colour from the session environment rather than from a
probe of its output stream, which is a network stream and never a terminal device. A client declaring no colour
or a dumb terminal SHALL get a monochrome board.

#### Scenario: A colour terminal

- **WHEN** a session requests a pseudo-terminal whose type is a colour terminal and sets no opt-out
- **THEN** the board is drawn in colour

#### Scenario: The client opts out

- **WHEN** a session carries NO_COLOR or the tix-scoped spelling set to a non-empty value
- **THEN** the board is drawn without colour

#### Scenario: A dumb or unnamed terminal

- **WHEN** a session names a dumb terminal, or names no terminal type at all
- **THEN** the board is drawn without colour
