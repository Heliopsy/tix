## ADDED Requirements

### Requirement: Single service contract

A single `Service` interface SHALL describe the complete product surface, and every access path, including the CLI, TUI, HTTP API, WebSocket stream, and web UI, SHALL perform its operations exclusively through that interface. No access path SHALL reach the storage layer directly.

#### Scenario: Operation available on every path

- **WHEN** a new operation is added to the service contract
- **THEN** it becomes available to every access path through the same method, with the same arguments and the same result shape

#### Scenario: No bypass of the contract

- **WHEN** any access path attempts an operation not present on the service contract
- **THEN** that operation does not exist in the product and the code does not build

#### Scenario: Contract is transport-neutral

- **WHEN** the service contract is inspected
- **THEN** no method signature refers to HTTP, CLI, terminal, or any other transport concept

### Requirement: Two implementations with rules in only one

There SHALL be exactly two implementations of the service contract: a local implementation that owns every business rule, and a remote client implementation that performs no validation and enforces no business rule whatsoever. The remote client SHALL only marshal requests and unmarshal responses and errors.

#### Scenario: Local implementation enforces a rule

- **WHEN** an invalid state transition is requested against the local implementation
- **THEN** the request is rejected with an invalid-input error before anything is written

#### Scenario: Remote client forwards without judging

- **WHEN** an invalid state transition is requested through the remote client
- **THEN** the request is sent to the server, the server's local implementation rejects it, and the client surfaces the server's error unchanged

#### Scenario: Server executes the same code

- **WHEN** an operation arrives over HTTP
- **THEN** it is executed by the same local implementation method that a direct-database caller would invoke

#### Scenario: No duplicated rule in the client

- **WHEN** the remote client is inspected
- **THEN** it contains no validation, no authorization check, and no transition legality logic

### Requirement: Single authorization enforcement point

Authorization SHALL be evaluated in exactly one place, the local service implementation. The HTTP layer SHALL perform authentication only, resolving identity and tenant, and SHALL NOT make authorization decisions. The remote client SHALL perform neither authentication decisions nor authorization decisions.

#### Scenario: HTTP layer authenticates only

- **WHEN** an authenticated request arrives for an operation the actor is not permitted to perform
- **THEN** the HTTP layer accepts the identity, the local service denies the operation, and a forbidden error is returned

#### Scenario: Unauthenticated request

- **WHEN** a request arrives with no credentials while authentication is required
- **THEN** an unauthenticated error is returned and no service operation is executed

#### Scenario: Same denial on both transports

- **WHEN** the same unpermitted operation is attempted locally and remotely by the same actor
- **THEN** both attempts are denied with the forbidden error

### Requirement: Authorization policy is always in the path

The authorization policy SHALL be evaluated for every operation in every mode, including no-auth mode. In no-auth mode the policy SHALL synthesize a local actor with full scope and allow every operation, rather than being skipped.

#### Scenario: No-auth mode still consults the policy

- **WHEN** an operation runs with authentication disabled
- **THEN** the policy is evaluated, returns an allow decision, and the operation proceeds

#### Scenario: Enabling authentication introduces no untested branch

- **WHEN** authentication is turned on for an installation that previously ran in no-auth mode
- **THEN** the same policy evaluation path is used, now returning real decisions, with no alternate code path activated

### Requirement: Actor and tenant travel in the request context

Actor identity and tenant scope SHALL be carried in the request context. Each transport SHALL populate them in its own way, and the service SHALL consume them identically regardless of transport.

#### Scenario: HTTP populates from credentials

- **WHEN** a request arrives with a valid API token
- **THEN** the actor and tenant resolved from that token are placed in the context and used by the service

#### Scenario: CLI populates from the active context

- **WHEN** a direct-database command runs under a configured context
- **THEN** the actor and tenant derived from that configuration are placed in the request context and used by the service

#### Scenario: Missing tenant scope is refused

- **WHEN** a service method is invoked with no tenant scope in the context
- **THEN** the operation fails rather than executing against an unscoped dataset

#### Scenario: Service reads only the context

- **WHEN** the service determines who is acting and which tenant is in scope
- **THEN** it takes both from the request context and from no transport-specific source

### Requirement: Error taxonomy

The service SHALL classify every failure into one of a fixed set of error kinds: not found, conflict, forbidden, unauthenticated, invalid input, lease expired, no task available, and precondition failed. Every error returned from the service SHALL carry exactly one of these kinds.

#### Scenario: Missing entity

- **WHEN** an operation references a task that does not exist within the tenant scope
- **THEN** a not-found error is returned

#### Scenario: Duplicate key

- **WHEN** a project is created with a key that already exists in the tenant
- **THEN** a conflict error is returned

#### Scenario: Empty queue

- **WHEN** a claim-next operation finds no eligible task
- **THEN** a no-task-available error is returned rather than an empty success or a not-found error

#### Scenario: Stale lease

- **WHEN** an operation presents a lease token that is no longer valid for the task
- **THEN** a lease-expired error is returned and no state is changed

#### Scenario: Unclassified failure is not permitted

- **WHEN** the service returns an error
- **THEN** it maps to one of the defined kinds and is never an opaque untyped failure

### Requirement: Deterministic error mapping across transports

Each service error kind SHALL map one-to-one to an HTTP status code and one-to-one to a CLI exit code, and those mappings SHALL be identical for every operation.

#### Scenario: HTTP mapping

- **WHEN** a not-found error is returned over HTTP
- **THEN** the response carries the status code assigned to not found, and the same status is used for every not-found error from any operation

#### Scenario: Remote client restores the kind

- **WHEN** the remote client receives an error response from the server
- **THEN** it reconstructs the original error kind so callers observe the same kind as a local caller would

#### Scenario: Forbidden and unauthenticated are distinct

- **WHEN** an actor is authenticated but not permitted, and separately when an actor presents no credentials
- **THEN** the two cases produce different error kinds and different status codes

### Requirement: Structured error detail

Errors SHALL carry a stable machine-readable code and a human-readable message, and SHALL identify the offending field or entity where one applies.

#### Scenario: Invalid field is named

- **WHEN** a create operation fails validation on a specific field
- **THEN** the error identifies that field

#### Scenario: Stable code for scripting

- **WHEN** the same failure occurs across releases
- **THEN** the machine-readable code is unchanged even if the human-readable message is reworded

### Requirement: Single transaction per mutation

Every mutation SHALL write its domain rows, its audit entry, and its outbox event within one database transaction. A mutation SHALL NOT be observable unless all three are committed together.

#### Scenario: Atomic success

- **WHEN** a task status is changed
- **THEN** the task row, the corresponding audit entry, and the corresponding outbox event are all committed together

#### Scenario: Rollback leaves nothing behind

- **WHEN** a mutation fails partway through
- **THEN** no domain row, no audit entry, and no outbox event from that mutation is visible

#### Scenario: No event without an audit entry

- **WHEN** the events table is compared with the audit log for any committed mutation
- **THEN** each mutation has both, and neither exists without the other

### Requirement: Transaction boundaries exclude external I/O

A write transaction SHALL NOT perform network calls or other external I/O. Side effects such as webhook delivery SHALL occur after the transaction commits.

#### Scenario: Webhook fired after commit

- **WHEN** a mutation that triggers a webhook is performed
- **THEN** the transaction commits first and the outbound request is attempted afterwards

#### Scenario: Slow endpoint does not hold a lock

- **WHEN** a configured webhook endpoint is unresponsive
- **THEN** the mutation still commits promptly and no write lock is held while waiting

### Requirement: Optimistic concurrency

Updates that carry an expected version SHALL be applied only if the stored version still matches. A mismatch SHALL produce a conflict error and SHALL NOT overwrite the newer state.

#### Scenario: Stale update rejected

- **WHEN** two callers read the same task and both submit an update with the version they read
- **THEN** the first update succeeds and the second is rejected with a conflict error

#### Scenario: Version advances on write

- **WHEN** an update succeeds
- **THEN** the entity's version changes so a subsequent update with the old version is rejected

#### Scenario: Unconditional update is explicit

- **WHEN** an update is submitted without an expected version
- **THEN** it applies to the current state, and this behaviour is only reachable by omitting the version deliberately

### Requirement: Service methods are idempotent where declared

Operations documented as idempotent SHALL produce the same resulting state and the same success result when repeated with the same inputs, and SHALL NOT emit duplicate events for a repeated no-op.

#### Scenario: Repeating an idempotent operation

- **WHEN** an idempotent operation is invoked twice with identical inputs
- **THEN** the resulting state after the second invocation matches the state after the first

#### Scenario: No spurious events

- **WHEN** an idempotent operation results in no state change
- **THEN** no outbox event is emitted for that invocation

### Requirement: Operation registry spans every surface

Every operation SHALL be declared once in a registry that records its stable product name, the service method it maps to, and its bindings on each access path: the CLI command path, the HTTP route, the browser route and the template it renders, and the terminal view it is reachable from where one exists. The CLI, the HTTP API, and the browser SHALL be the surfaces every operation is required to reach; the terminal interface SHALL be recorded where it applies but SHALL NOT be required. The registry SHALL be the single source of truth for what operations exist, and no surface SHALL expose an operation the registry does not declare.

#### Scenario: Every service method is registered

- **WHEN** the registry is checked against the exported service interface
- **THEN** every exported service method appears in the registry exactly once

#### Scenario: Entries name their bindings

- **WHEN** a registry entry is read
- **THEN** it names the CLI command, the HTTP route, and the browser route that expose that operation, or carries an exemption for each required surface it omits

#### Scenario: Lookup by service method

- **WHEN** an operation is looked up by the service method it maps to
- **THEN** the registry returns the single entry declaring that method

### Requirement: Exemptions distinguish inapplicability from a known gap

An operation absent from a bound surface SHALL carry an exemption recording the surface and a written reason. An exemption SHALL further record whether the absence is permanent, because the surface cannot serve the operation, or a known defect held open until the binding is written. A defect exemption SHALL be enumerable on its own, so the set of missing bindings can be read off the registry rather than discovered by hand.

#### Scenario: Permanent exemption

- **WHEN** an operation a browser cannot serve carries an exemption naming the browser surface and its reason
- **THEN** the parity test accepts the absence and the exemption is not counted as a defect

#### Scenario: Known gap is flagged as one

- **WHEN** an operation lacks a binding that ought to exist and records an exemption for it
- **THEN** the exemption is marked as a gap and its reason says so, and the parity test fails when the marking and the reason disagree

#### Scenario: Gaps are enumerable

- **WHEN** the registry's gaps are listed
- **THEN** every exemption marked as a defect is returned with the service method carrying it, and no permanent exemption is included

#### Scenario: Exemption without a reason is rejected

- **WHEN** an operation records an exemption carrying no written reason
- **THEN** the parity test fails

### Requirement: Attribution of every mutation

Every mutation SHALL be attributed to the acting identity in its audit entry and its event, including synthetic identities used for automated operations.

#### Scenario: Human actor recorded

- **WHEN** a user changes a task
- **THEN** the audit entry and the event name that user as the actor

#### Scenario: System actor recorded

- **WHEN** an automated operation such as an import performs mutations
- **THEN** they are attributed to a system actor rather than to whoever triggered the run

#### Scenario: No anonymous mutation

- **WHEN** any mutation is committed
- **THEN** an actor is recorded, including the synthetic local actor used in no-auth mode
