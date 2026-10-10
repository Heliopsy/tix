## ADDED Requirements

### Requirement: Single connection resolution point

The choice between the local implementation and the remote client SHALL be made in exactly one place, and every access path SHALL obtain its service instance from it. No command SHALL construct a service instance by its own logic.

#### Scenario: All commands share the resolution

- **WHEN** any command needs a service instance
- **THEN** it obtains one from the single resolver and does not decide local versus remote itself

#### Scenario: Resolution result is reported

- **WHEN** a command is run with diagnostics enabled
- **THEN** the resolver reports whether the connection is local or remote and which source determined it

### Requirement: Connection resolution order

The resolver SHALL select a target in the following descending order: an explicit `--server` or `--db` override, then an explicit `--ctx` or `$TIX_CONTEXT`, then an auto-detected per-directory context, then `current_context` from the configuration file, then a default SQLite file created and migrated on first use.

#### Scenario: Raw override wins

- **WHEN** `--server` is passed while a per-directory context and a current context both exist
- **THEN** the named server is used

#### Scenario: Explicit context beats discovery

- **WHEN** `--ctx work` is passed from a directory containing `.tix.yaml` naming a different context
- **THEN** the `work` context is used

#### Scenario: Environment context beats the config file

- **WHEN** `TIX_CONTEXT` is set and `current_context` names a different context
- **THEN** the context from `TIX_CONTEXT` is used

#### Scenario: Discovered context beats the config file

- **WHEN** no flag or environment context is set and a per-directory context is discovered
- **THEN** the discovered context is used in preference to `current_context`

#### Scenario: Fallback to the default database

- **WHEN** no override, no explicit context, no discovered context, and no current context is present
- **THEN** the default SQLite file is used, created and migrated if it does not already exist

### Requirement: Mutually exclusive target overrides

`--server` and `--db` SHALL NOT be accepted together, because a target is either a server or a database and never both.

#### Scenario: Both overrides supplied

- **WHEN** `--server` and `--db` are passed in the same invocation
- **THEN** the command fails with a usage error and performs no operation

#### Scenario: Override replaces the context endpoint

- **WHEN** `--db` is passed while the selected context names a server URL
- **THEN** the database named by `--db` is used and the context's server URL is ignored

### Requirement: Zero-configuration operation

A user SHALL be able to perform a complete operation on a machine that has never run tix, with no configuration file, no server, and no prior setup command.

#### Scenario: First command on a fresh machine

- **WHEN** a task is added on a machine with no tix configuration and no existing database
- **THEN** the default database is created and migrated, the task is created, and the command exits successfully

#### Scenario: Data persists across invocations

- **WHEN** a second command lists tasks after the first invocation created one
- **THEN** the previously created task is returned

#### Scenario: No server required

- **WHEN** no tix server is running anywhere
- **THEN** every operation that does not inherently require a remote target still succeeds locally

### Requirement: Transport equivalence

Any operation invoked through the local implementation and through the remote client SHALL produce identical results and identical error kinds for identical inputs and identical actor and tenant context. This equivalence SHALL be verified by an automated test that exercises every operation on the service contract against both transports. An operation that is not exercised SHALL carry a written exemption naming the reason it is not, and an operation with neither SHALL fail the build. The obligation is on the operations, not on whichever scenarios happen to be written.

#### Scenario: Identical success results

- **WHEN** the same create operation is performed locally and remotely with the same inputs
- **THEN** the resulting entities are equivalent in every field other than values that are inherently unique per creation

#### Scenario: Identical error kinds

- **WHEN** the same invalid operation is attempted locally and remotely
- **THEN** both return the same error kind and the same machine-readable code

#### Scenario: Equivalence suite covers every operation

- **WHEN** the equivalence test suite runs
- **THEN** every operation on the service contract is exercised on both transports, or carries an exemption stating why it is not

#### Scenario: A new operation without a scenario fails the build

- **WHEN** an operation is added to the service contract and no equivalence scenario exercises it and no exemption names it
- **THEN** the equivalence suite fails and names the operation

#### Scenario: An exemption states its reason

- **WHEN** an operation is exempted from the equivalence suite
- **THEN** the exemption carries a reason, and an empty reason fails the build

#### Scenario: Divergence fails the build

- **WHEN** an operation behaves differently on one transport than the other
- **THEN** the equivalence test fails

### Requirement: Remote client surfaces server errors faithfully

The remote client SHALL reconstruct the error kind and machine-readable code returned by the server and SHALL NOT substitute a generic transport failure for an application error.

#### Scenario: Application error preserved

- **WHEN** the server returns a conflict for a duplicate key
- **THEN** the caller of the remote client observes a conflict error, not a generic request failure

#### Scenario: Genuine transport failure is distinguishable

- **WHEN** the server is unreachable
- **THEN** the caller receives an error that identifies a connectivity problem and is distinguishable from any service error kind

### Requirement: Events are produced regardless of whether a server runs

Outbox event rows SHALL be committed by every mutation whether or not a server process is running. A server connected to the same database SHALL pick up those events and deliver them to its subscribers.

#### Scenario: CLI write with no server

- **WHEN** a mutation is performed by the CLI directly against the database while no server is running
- **THEN** the outbox event row is committed

#### Scenario: Server elsewhere picks up the event

- **WHEN** a server is running against the same database and a direct-database CLI performs a mutation
- **THEN** the server's subscribers receive the corresponding event without the CLI contacting the server

#### Scenario: Events survive process death

- **WHEN** the writing process exits immediately after committing
- **THEN** the event remains available for later delivery

### Requirement: Bounded latency for direct-database subscription

A subscription served directly from the database SHALL observe committed events within a bounded polling interval and SHALL deliver only committed events.

#### Scenario: Event observed within the bound

- **WHEN** a mutation commits while a direct-database subscriber is active
- **THEN** the subscriber receives the event within the configured polling bound

#### Scenario: Uncommitted work is never delivered

- **WHEN** a transaction writes an event row and then rolls back
- **THEN** no subscriber receives an event for it

#### Scenario: Ordering is preserved

- **WHEN** several mutations commit in sequence
- **THEN** the subscriber receives their events in commit order

### Requirement: Webhook delivery without a server

Webhook delivery rows SHALL be drained by whichever process is available. Delivery mode SHALL be configurable as inline, server, or off. When nothing drains them, pending deliveries SHALL remain pending and SHALL NOT be discarded.

#### Scenario: Inline drain from the CLI

- **WHEN** delivery mode is inline and a CLI mutation commits with a matching webhook endpoint configured
- **THEN** the CLI attempts a bounded opportunistic drain after the commit

#### Scenario: Server mode leaves delivery to the server

- **WHEN** delivery mode is server
- **THEN** the CLI commits the delivery row and does not attempt delivery itself

#### Scenario: Off mode records but does not deliver

- **WHEN** delivery mode is off
- **THEN** delivery rows are still committed and no delivery attempt is made by that process

#### Scenario: Nothing is lost when nothing drains

- **WHEN** no process drains deliveries for an extended period
- **THEN** the pending rows remain and are delivered once a draining process runs

#### Scenario: Backlog is observable

- **WHEN** pending deliveries have accumulated
- **THEN** the diagnostic command reports the backlog size

### Requirement: Delivery attempts never block a commit

A drain performed by a short-lived process SHALL be bounded in time and in number of attempts, and SHALL NOT prevent the originating command from completing.

#### Scenario: Unresponsive endpoint

- **WHEN** an inline drain targets an endpoint that does not respond
- **THEN** the drain gives up within its bound and the command still exits successfully

#### Scenario: Command exit code unaffected by delivery

- **WHEN** an inline delivery attempt fails
- **THEN** the mutation is still reported as successful and the failure is visible in the delivery records rather than in the command's exit status

### Requirement: Remote target requires credentials when the server demands them

When the resolved target is a server that requires authentication, operations SHALL fail with an unauthenticated error if no token is available from the resolved configuration.

#### Scenario: Missing token

- **WHEN** a context names a server that requires authentication and supplies no token
- **THEN** operations fail with an unauthenticated error that states how to supply a token

#### Scenario: Token supplied by environment

- **WHEN** the token is absent from the context but present in the environment
- **THEN** the environment token is used and the operation proceeds

### Requirement: Safe default database location

The default SQLite database SHALL be created at a deterministic per-user location, and tix SHALL refuse to use a SQLite database file that resides on a network filesystem.

#### Scenario: Deterministic default path

- **WHEN** the default database is used with no configuration
- **THEN** the same path is chosen on every invocation for that user

#### Scenario: Network filesystem refused

- **WHEN** the resolved SQLite path is detected to be on a network filesystem
- **THEN** the command fails with an error explaining the corruption risk and naming the path
