## MODIFIED Requirements

### Requirement: The key fingerprint is the identity

The listener SHALL resolve a proven public key's fingerprint to the actor the session runs as, and SHALL
require a public key: no password and no other credential SHALL authenticate a session.

In demo mode the listener SHALL accept any key without it having been registered beforehand. Otherwise the
listener SHALL accept only a fingerprint enrolled against an actor and not revoked, and SHALL refuse any
other key.

#### Scenario: An unknown key is accepted in demo mode

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

### Requirement: Bind and target safety

The listener SHALL refuse to bind a non-loopback address unless an explicit opt-out is supplied.

In demo mode the listener SHALL refuse to run against the zero-configuration database and SHALL require a
database target of its own, because a listener that provisions a tenant for any stranger must not share a
store with real work. When serving enrolled keys the listener SHALL accept the configured target like any
other command, because serving the real store is the purpose.

#### Scenario: Public bind without the opt-out

- **WHEN** a non-loopback listen address is configured without the explicit opt-out
- **THEN** the listener refuses to start and explains that the choice must be explicit

#### Scenario: Demo mode against the zero-configuration store

- **WHEN** the listener is started in demo mode without a database or server being named anywhere
- **THEN** it refuses to start, explaining that the zero-configuration store holds real work and that demo
  mode needs a database of its own

#### Scenario: Enrolled mode against the configured target

- **WHEN** the listener is started without demo mode and without an explicit database
- **THEN** it opens the configured target like any other command

## ADDED Requirements

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
