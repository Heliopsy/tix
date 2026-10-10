# ssh-access Specification

## Purpose

Serves the terminal interface over SSH from the tix process itself, with its own identity, authorization and capacity rules.

## Requirements

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

The listener SHALL resolve a proven public key's fingerprint to the actor the session runs as, and SHALL
require a public key: no password and no other credential SHALL authenticate a session.

In demo mode the listener SHALL accept any key without it having been registered beforehand. Otherwise the
listener SHALL accept only a fingerprint enrolled against an actor and not revoked, and SHALL refuse any
other key.

#### Scenario: An unknown key is accepted

- **WHEN** the listener runs in demo mode and a client connects with a public key it has never seen
- **THEN** the connection is authenticated and a session begins

#### Scenario: An unknown key is refused when serving enrolled keys

- **WHEN** the listener is not in demo mode and a client connects with a fingerprint that is not enrolled
- **THEN** the connection is refused, and the refusal does not reveal whether any tenant exists

#### Scenario: A revoked key is refused

- **WHEN** a client connects with a fingerprint that was enrolled and has since been revoked
- **THEN** the connection is refused

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

The listener SHALL refuse to bind a non-loopback address unless an explicit opt-out is supplied.

In demo mode the listener SHALL refuse to run against the zero-configuration database and SHALL require a
database target of its own, because a listener that provisions a tenant for any stranger must not share a
store with real work. When serving enrolled keys the listener SHALL accept the configured target like any
other command, because serving the real store is the purpose.

#### Scenario: Public bind without the opt-out

- **WHEN** a non-loopback listen address is configured without the explicit opt-out
- **THEN** the listener refuses to start and explains that the choice must be explicit

#### Scenario: No database named

- **WHEN** the listener is started in demo mode without a database or server being named anywhere
- **THEN** it refuses to start, explaining that the zero-configuration store holds real work and that demo
  mode needs a database of its own

#### Scenario: Enrolled mode against the configured target

- **WHEN** the listener is started without demo mode and without an explicit database
- **THEN** it opens the configured target like any other command

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

### Requirement: Every listener setting is a configuration key

Every setting of the listener SHALL be a configuration key with a generated environment variable, in addition to
its flag, and SHALL follow the documented layer order: flag, environment, `.env`, configuration file, default. A
flag the operator did not give SHALL NOT outrank a configured value. A duration SHALL be expressed as a duration
rather than as a string or a count of seconds. A value that cannot be used SHALL be refused at startup, naming
the key.

#### Scenario: Configured and not flagged

- **WHEN** the listener's address, limits and timeouts are set in a configuration file and no flag names them
- **THEN** the listener runs with those values

#### Scenario: An environment variable beats the file

- **WHEN** a key is set both in the configuration file and in the matching `TIX_SSH_*` variable
- **THEN** the variable's value is used

#### Scenario: A flag beats everything

- **WHEN** a key is set in the environment and the matching flag is also given
- **THEN** the flag's value is used, and keys whose flags were not given keep their configured values

#### Scenario: A value that cannot be used

- **WHEN** a negative window, a negative cap, or a per-key session cap above the listener's own cap is configured
- **THEN** the process refuses to start and names the offending key

### Requirement: A client that has gone is detected

The listener SHALL send a keepalive request to each session's client at a configurable interval, and SHALL close
the connection once a configurable number of them go unanswered. Keepalive traffic SHALL NOT count as activity
for the idle timeout. The defaults SHALL detect an absent client in appreciably less time than the idle timeout.

#### Scenario: The client vanishes without disconnecting

- **WHEN** a session's client stops answering without closing the connection
- **THEN** the connection is closed after the configured number of unanswered keepalives, releasing its session
  slot and any lease it held

#### Scenario: Keepalives do not keep an idle session alive

- **WHEN** a client answers every keepalive but nobody types for the idle timeout
- **THEN** the session is closed, because idleness is measured from the input the interface received and never
  from traffic

#### Scenario: A live client is left alone

- **WHEN** a client answers its keepalives
- **THEN** the session continues

### Requirement: Caps on concurrent sessions

The listener SHALL cap how many sessions one key holds at once and how many the listener holds in total, both
configurable. A session over either cap SHALL be refused with a message the client can read that names the
limit. The refusal SHALL NOT differ according to whether the key has been seen before.

#### Scenario: One key opens too many sessions

- **WHEN** a key already holding its limit of sessions opens another
- **THEN** the session is refused, on the session's error stream, with a message naming the limit, and the
  existing sessions are unaffected

#### Scenario: The listener is at its total

- **WHEN** the listener holds its limit of sessions and any key connects
- **THEN** the session is refused with a message naming that limit

#### Scenario: A refusal tells a stranger nothing

- **WHEN** a full listener refuses a key it has a sandbox for and a key it has never seen
- **THEN** both refusals are identical, and neither required a lookup to produce

### Requirement: Only a resolved identity occupies listener capacity

The SSH listener SHALL NOT let a key it has not resolved to an actor occupy a share of the listener-wide
session cap. A session slot SHALL be taken only after the presented fingerprint has been resolved to the
actor the session runs as.

The capacity refusal SHALL be decided before the fingerprint is looked up, so that a listener with no slot
left refuses every key at the same point in the exchange, with the same message, having done the same work.

#### Scenario: A stranger mid-lookup does not hold the last slot

- **WHEN** a key that is not enrolled has authenticated and is still being looked up on a listener whose
  session cap is one, and an enrolled key then opens a session
- **THEN** the enrolled session is served, and the unenrolled key is refused holding no slot

#### Scenario: A full listener answers an enrolled and an unenrolled key alike

- **WHEN** the listener is at its session cap and a key opens a session
- **THEN** it is refused for capacity before the fingerprint is looked up, whether or not that fingerprint
  is enrolled

#### Scenario: The cap still refuses an enrolled key it cannot serve

- **WHEN** the listener is at its session cap because enrolled sessions are holding every slot, and another
  enrolled key opens a session
- **THEN** that session is refused with the capacity message

#### Scenario: A demo key is refused nothing between the two steps

- **WHEN** the listener runs in demo mode, where every key resolves to a sandbox of its own
- **THEN** a key that passed the capacity check is given a session slot

### Requirement: Enrolling a public key against an actor

A public key SHALL be enrollable against an actor in a tenant, recording its fingerprint, the key itself and
an optional label. A fingerprint SHALL be unique within a tenant. Enrolment, listing and revocation SHALL be
reachable from the command line, the HTTP API and the web interface.

#### Scenario: A key is enrolled

- **WHEN** a public key is enrolled against an actor
- **THEN** its fingerprint is recorded against that actor in that tenant, and the fingerprint reported matches
  the one an SSH client prints for the same key

#### Scenario: The same key twice in one tenant

- **WHEN** a fingerprint already enrolled in a tenant is enrolled again in that tenant
- **THEN** the request is refused as a conflict

#### Scenario: The same key in two tenants

- **WHEN** a fingerprint enrolled in one tenant is enrolled against an actor in a second tenant
- **THEN** both enrolments exist, each scoped to its own tenant

#### Scenario: Input that is not a public key

- **WHEN** enrolment is given text that is not an SSH public key
- **THEN** it is refused with a message naming the expected format, and nothing is recorded

### Requirement: Revoking an enrolled key

An enrolled key SHALL be revocable, after which it SHALL NOT authenticate. Revocation SHALL NOT terminate
sessions the key already holds; those sessions SHALL remain bounded by the idle timeout.

#### Scenario: A revoked key cannot reconnect

- **WHEN** a key is revoked and its holder connects again
- **THEN** the connection is refused

#### Scenario: Revocation is visible

- **WHEN** enrolled keys are listed after a revocation
- **THEN** the revoked key is reported as revoked rather than omitted

### Requirement: The username selects the tenant

When a fingerprint is enrolled in more than one tenant, the SSH username SHALL select which. A neutral
default username SHALL mean "the only tenant this key is enrolled in", and SHALL be ambiguous when there is
more than one.

#### Scenario: Enrolled in one tenant

- **WHEN** a fingerprint enrolled in exactly one tenant connects with the neutral default username
- **THEN** the session opens in that tenant

#### Scenario: Enrolled in several, tenant named

- **WHEN** a fingerprint enrolled in several tenants connects with a username naming one of them
- **THEN** the session opens in the named tenant

#### Scenario: Enrolled in several, tenant not named

- **WHEN** a fingerprint enrolled in several tenants connects with the neutral default username
- **THEN** the connection is refused with a message naming the tenant keys that may be used as the username,
  and no tenant is chosen on the holder's behalf

#### Scenario: Username names a tenant the key is not enrolled in

- **WHEN** a client connects with a username naming a tenant its fingerprint is not enrolled in
- **THEN** the connection is refused

### Requirement: Enrolled keys are tenant-scoped data

An enrolled key SHALL belong to exactly one tenant and SHALL NOT be readable, listable or addressable from
another tenant through any surface, including when the underlying engine enforces row-level security.

#### Scenario: Another tenant cannot read a key

- **WHEN** a caller scoped to one tenant lists or addresses enrolled keys belonging to another
- **THEN** nothing belonging to the other tenant is returned

### Requirement: Serving the web interface and SSH from one process

The serve command SHALL accept an optional SSH listen address and, when one is given, SHALL run the SSH
listener alongside the HTTP server in the same process, over the same database, under the same shutdown. No
SSH listener SHALL be started unless an address is given.

#### Scenario: Both listeners run

- **WHEN** the serve command is given an SSH listen address
- **THEN** the HTTP server and the SSH listener both accept connections, and a change made over one is visible
  to the other

#### Scenario: No address given

- **WHEN** the serve command is run without an SSH listen address
- **THEN** no SSH listener is started and no SSH port is bound

#### Scenario: The SSH port is unavailable

- **WHEN** the configured SSH listen address is already in use
- **THEN** the process fails at startup, before the HTTP server begins accepting requests

#### Scenario: Shutdown drains both

- **WHEN** the process is asked to shut down while an SSH session and an HTTP request are both in flight
- **THEN** both are given the shutdown timeout to finish, and an SSH session still open when it expires is
  closed with a message

### Requirement: Colour depth follows each session's own client

Each session SHALL resolve its colour depth from its own client's terminal, and one session's terminal SHALL
NOT determine how another session renders. A client whose terminal cannot show colour SHALL NOT be sent
colour, whatever any other session is using.

#### Scenario: A colour client and a plain one, at once

- **WHEN** a client on a colour terminal and a client on a terminal that cannot show colour hold sessions at
  the same time
- **THEN** the colour client is sent colour and the plain client is sent none

#### Scenario: Depths that differ beyond the palette in use

- **WHEN** two clients at different colour depths hold sessions at the same time and a colour outside the
  interface's own palette is rendered
- **THEN** each session renders that colour at its own client's depth

### Requirement: Only a canonical public key is stored

Enrolment SHALL parse the submitted text as an SSH public key and SHALL store the canonical form of the
parsed key, never the submitted bytes. The recorded fingerprint SHALL be derived from the parsed key. No
comment accompanying the submission SHALL be stored.

A submission carrying authorized_keys options, such as a command directive or a restriction, SHALL be
refused rather than accepted with the options discarded. Silently dropping a directive would leave the
submitter believing they had applied a restriction that does not exist, which is a worse outcome than
refusing the submission and saying so.

#### Scenario: Options are refused, not quietly dropped

- **WHEN** enrolment is given an authorized_keys line carrying options such as a command directive
- **THEN** it is refused, the message says options are not supported, and nothing is recorded

#### Scenario: A comment is not stored

- **WHEN** a key is enrolled from a line ending in a comment, as ssh-keygen writes it
- **THEN** the enrolment succeeds and the stored key is the key alone, without the comment

#### Scenario: Two spellings of one key are one identity

- **WHEN** the same key is enrolled twice in a tenant, differing only in comment or surrounding whitespace
- **THEN** the second enrolment is refused as a conflict, because both resolve to the same fingerprint

#### Scenario: Trailing content is refused

- **WHEN** enrolment is given text carrying more than one key, or content after the key that is not a comment
- **THEN** it is refused and nothing is recorded

### Requirement: Enrolment cannot grant authority the caller lacks

An actor SHALL enrol and revoke its own keys. Enrolling a key against another actor SHALL require authority
over that actor within the tenant. Enrolment SHALL NOT be a path to an actor, a tenant or a role the caller
could not otherwise reach.

#### Scenario: Enrolling against another actor without authority

- **WHEN** an actor without authority over other actors enrols a key against a different actor
- **THEN** the request is refused and no key is recorded

#### Scenario: Enrolment does not cross the tenant boundary

- **WHEN** an administrator of one tenant enrols a key naming an actor of another tenant
- **THEN** the request is refused, and the key does not authenticate into the other tenant

#### Scenario: A key inherits no authority of its own

- **WHEN** a key enrolled against an actor is used to open a session
- **THEN** the session holds exactly the authority that actor holds, and the key grants nothing beyond it

### Requirement: Refusals disclose nothing about what exists

A refused connection SHALL NOT reveal whether a tenant exists, whether a fingerprint is enrolled anywhere, or
which actor a key belongs to. Only a caller whose key is already enrolled in the tenants named SHALL be told
about them.

#### Scenario: An unenrolled key learns nothing

- **WHEN** a key that is enrolled nowhere connects naming a tenant that exists, and again naming one that does
  not
- **THEN** both connections are refused the same way, and neither response distinguishes the two cases

#### Scenario: The ambiguity message names only the caller's own tenants

- **WHEN** a fingerprint enrolled in several tenants is refused for ambiguity
- **THEN** the message names only tenants that fingerprint is enrolled in, and no others

### Requirement: The cross-tenant lookup is narrowed before anything else happens

Resolution by fingerprint across tenants SHALL be the only unscoped access to enrolled keys, SHALL be confined
to the authentication path, and SHALL produce a single tenant scope before any further work is done. No
session SHALL run without a tenant scope.

#### Scenario: Ambiguity does not open a session

- **WHEN** a fingerprint resolves to enrolments in more than one tenant and no tenant is named
- **THEN** no session is opened, no tenant is entered, and no record other than the refusal is written

#### Scenario: Recording use is not a trust decision

- **WHEN** recording that a key was used fails
- **THEN** the outcome of the authentication is unchanged by that failure
