# service-layer Specification

## Purpose

Holds the single Service interface every access path goes through, so business rules, validation and authorization cannot differ between CLI, TUI, API and browser.

## Requirements

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

### Requirement: A successful read through the contract returns a record

A method of that interface declared as returning a pointer and an error SHALL return a non-nil
pointer whenever it returns a nil error. There is no third answer: an absent record is an error, and
a read that succeeded has a record to return. A caller MAY therefore branch on the error alone.

#### Scenario: A read that succeeded returns a record

- **WHEN** a service method declared to return a pointer and an error returns a nil error
- **THEN** the pointer it returns is not nil

### Requirement: A broken read guarantee is reported as an internal fault

An access path SHALL NOT be required to defend that guarantee at each of its own call sites. Where a
path consumes such a read, a violation SHALL be reported as an internal fault naming the record that
was missing, rather than reaching a dereference.

#### Scenario: An implementation that breaks the guarantee is reported, not dereferenced

- **GIVEN** an implementation of the service contract that returns a nil pointer together with a nil
  error
- **WHEN** a screen that reads a single record through it is served
- **THEN** the request fails with an internal fault that names the record that was missing
- **AND** no dereference of the absent record is attempted

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

Every operation SHALL be declared once in a registry that records its stable product name, the service method it maps to, and its bindings on each access path: the CLI command path, the HTTP route, the browser route and the template it renders, and the terminal view it is reachable from where one exists.

#### Scenario: Every service method is registered

- **WHEN** the registry is checked against the exported service interface
- **THEN** every exported service method appears in the registry exactly once

#### Scenario: Lookup by service method

- **WHEN** an operation is looked up by the service method it maps to
- **THEN** the registry returns the single entry declaring that method

### Requirement: The registry decides which surfaces an operation must reach

The CLI, the HTTP API, and the browser SHALL be the surfaces every operation is required to reach; the terminal interface SHALL be recorded where it applies but SHALL NOT be required. The registry SHALL be the single source of truth for what operations exist, and no surface SHALL expose an operation the registry does not declare.

#### Scenario: Entries name their bindings

- **WHEN** a registry entry is read
- **THEN** it names the CLI command, the HTTP route, and the browser route that expose that operation, or carries an exemption for each required surface it omits

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

### Requirement: An actor identifier can be resolved to an identity

The service SHALL offer an operation resolving an actor identifier of the caller's tenant to the
identity behind it. Because every reader of a record that names an actor needs it, the operation
SHALL require no scope beyond an authenticated caller, and it SHALL return the naming fields only --
identifier, kind, handle and display name -- never the actor's scopes, role or token. An identifier
belonging to another tenant SHALL be reported missing, like any other record of another tenant.

#### Scenario: A handle is returned

- **WHEN** an actor identifier of the caller's tenant is resolved
- **THEN** the actor's handle, kind and display name are returned

#### Scenario: Authority is not disclosed

- **WHEN** an actor is resolved
- **THEN** no scope, role or token identifier is returned with it

#### Scenario: A reader with no administrative scope can resolve

- **WHEN** an actor holding only read scopes resolves an identifier
- **THEN** the lookup succeeds

#### Scenario: Resolution is tenant scoped

- **WHEN** an identifier belonging to another tenant is resolved
- **THEN** the operation reports it missing and discloses nothing about it

#### Scenario: The system actor is named

- **WHEN** the identifier recording an import, a sweep or a prune is resolved
- **THEN** it resolves to the system identity rather than being reported missing

### Requirement: View arrangement operations

The service SHALL offer `ListGroupPrefs`, `SetGroupPref`, `OrderGroups` and `ResetGroupPrefs`,
each acting only on the caller's own preferences and requiring the `task:read` scope. A view SHALL
be `board`, `lanes` or `project:` followed by a project reference. A `lanes` group SHALL be a
project of the tenant; a `project:` group SHALL be a state of that project's workflow; a `board`
group SHALL be a state of some workflow of the tenant.

#### Scenario: A state the workflow lacks is refused

- **WHEN** a caller hides group `shipped` on `project:infra` whose workflow has no such state
- **THEN** the call fails as invalid and nothing is written

#### Scenario: A project-pinned token cannot arrange

- **WHEN** a token pinned to one project calls `SetGroupPref`
- **THEN** the call is refused as forbidden

### Requirement: Ordering view groups writes their positions in the order named

`OrderGroups` SHALL write positions 1..n for the groups named, in that order, and SHALL refuse a
duplicate. Every view arrangement operation SHALL be registered with CLI, HTTP and web bindings.

#### Scenario: Ordering lanes writes positions

- **WHEN** a caller orders the `lanes` view as `ops` then `infra`
- **THEN** `ops` is stored at position 1 and `infra` at position 2

### Requirement: Saved filter operations

The service SHALL offer `ListSavedFilters`, `GetSavedFilter`, `CreateSavedFilter`, `UpdateSavedFilter`,
`OrderSavedFilters` and `DeleteSavedFilter`. Each SHALL require `task:read`; changing or deleting a shared
filter the caller does not own SHALL require `tenant:admin` and SHALL otherwise be refused as forbidden.
Another actor's private filter SHALL be reported as not found.

#### Scenario: A member cannot change somebody's shared filter

- **GIVEN** a shared filter owned by alice
- **WHEN** bob, a member, renames it
- **THEN** the call is refused as forbidden and nothing is written

#### Scenario: An administrator can

- **GIVEN** a shared filter owned by alice
- **WHEN** an administrator renames it
- **THEN** the new name is stored and an audit entry records it

#### Scenario: Ordering moves only the caller's own filters

- **WHEN** a reader orders filters naming one somebody else shared
- **THEN** the call is refused as not found

### Requirement: Saved filters are addressed, named and written consistently

A filter SHALL be addressed by identifier, then by the caller's own name, then by a shared name. Names
SHALL be unique per owner without regard to case. A saved expression SHALL be refused as invalid when the
filter parser cannot read it. Each saved filter change SHALL write its rows, an audit entry and an event in
one transaction, and a change that changes nothing SHALL write nothing.

#### Scenario: A name taken in another case is refused

- **GIVEN** a reader with a saved filter named "Ops"
- **WHEN** they create another saved filter named "OPS"
- **THEN** the call is refused as a conflict

### Requirement: me names the caller in a filter

A task listing SHALL read `me` (any case) in the assignee, creator and claimant terms, included or
excluded, as the calling actor, without changing the filter the caller passed.

#### Scenario: assignee:me lists the caller's tasks

- **GIVEN** a task assigned to alice and one assigned to bob
- **WHEN** alice lists tasks with `assignee:me`
- **THEN** only her task is listed

### Requirement: Calendar feed operations

The service SHALL offer `GetCalendarFeed`, `CreateCalendarFeed`, `RotateCalendarFeed`,
`SetCalendarFeedFilter` and `RevokeCalendarFeed` on the caller's own feed behind the `calendar.feed`
action, which needs `task:read` and is not confinable to a project, and `ReadCalendarFeed`, which
takes the secret and no actor.

#### Scenario: Rotation keeps the filter

- **GIVEN** a feed with the filter `tag:ops`
- **WHEN** its owner rotates it
- **THEN** the new URL answers with the same filter and the old URL answers 404

### Requirement: Calendar feed changes are refused on conflict and never carry the secret

Creating a feed while one is live SHALL be refused as a conflict, and an actor with no membership
in the tenant SHALL be refused. Each calendar feed change SHALL write its row, an audit entry and a
`calendar_feed.*` event in one transaction, and no event or audit entry SHALL carry the secret or
its hash.

#### Scenario: A second feed is a conflict

- **GIVEN** an actor with a live feed
- **WHEN** they create another
- **THEN** the call fails as a conflict and the first feed's URL still answers

### Requirement: Changes that concern somebody create inbox items

The service SHALL create an inbox item, in the same transaction as the change, when an actor is
mentioned as `@handle` in a comment or a task description (`mention`), when a task is assigned to an
actor (`assigned`), when somebody comments on a task an actor is assigned to or has starred
(`comment`), and when a task an actor is assigned to changes status (`status`).

#### Scenario: A mention reaches the person named

- **WHEN** ada comments "@bob can you look" on a task
- **THEN** bob's inbox holds one unread `mention` item naming the task, ada and the comment

#### Scenario: Watchers hear about comments

- **GIVEN** carol starred a task assigned to bob
- **WHEN** ada comments on it
- **THEN** bob and carol each receive a `comment` item

#### Scenario: A status change reaches the assignee

- **GIVEN** a task assigned to bob
- **WHEN** ada moves it from `todo` to `doing`
- **THEN** bob receives a `status` item carrying `todo` and `doing`

### Requirement: Inbox items leave out the actor, repeats, unknown handles and muted kinds

The service SHALL NOT create an inbox item for the actor who made the change. One change SHALL give
one recipient at most one item per task, keeping the strongest reason in the order mention, assigned,
status, comment. Editing a comment or a description SHALL mention only handles the earlier text did
not. A handle that names no actor SHALL be ignored. A kind the recipient muted SHALL create no item
for them.

#### Scenario: Your own actions do not notify you

- **WHEN** ada assigns a task to herself and comments "@ada note to self"
- **THEN** ada's inbox is unchanged

#### Scenario: The strongest reason wins

- **GIVEN** bob is assigned to a task and has starred it
- **WHEN** ada comments on it mentioning @bob
- **THEN** bob receives exactly one item, of kind `mention`

#### Scenario: A muted kind is silent

- **GIVEN** bob muted `comment`
- **WHEN** ada comments on a task bob is assigned to
- **THEN** bob receives nothing

### Requirement: Reading and muting the inbox

The service SHALL let an actor list their own items newest first with keyset pagination, optionally
only unread ones; count their unread items; mark one item read or unread; mark every item read; and
read and replace the set of kinds they muted. An item of another actor SHALL be not found.

#### Scenario: Somebody else's item

- **WHEN** ada marks bob's item read
- **THEN** the call fails as not found and the item stays unread

### Requirement: Inbox changes are audited and need task read authority

Each inbox change SHALL write an audit entry and an `inbox.updated` event in one transaction, and a
change that changes nothing SHALL write nothing. The inbox methods SHALL need `task:read` and SHALL
NOT be available to a project-pinned token.

#### Scenario: Marking everything read

- **GIVEN** bob has three unread items
- **WHEN** bob marks everything read
- **THEN** his unread count is 0 and one audit entry records it

### Requirement: Invite operations

The service SHALL offer `CreateInvite`, `ListInvites`, `RevokeInvite` and `ReissueInvite` behind
`user:admin`, and `PreviewInvite` and `AcceptInvite`, which take the token and no actor. Each
change SHALL write its rows, an audit entry and a `user_invite.*` event in one transaction, and no
audit entry, event or log line SHALL carry the token or its hash.

#### Scenario: The token is nowhere in the trail

- **WHEN** an invite is created, re-issued and accepted
- **THEN** no audit entry, event payload or request log line contains any of the tokens

### Requirement: Whom an invite may name

An invite for a new address SHALL create the user, its actor and its membership with no password.
An address of this tenant's user holding no membership SHALL be granted the membership without
touching the password. An address belonging to a member who already has a password, or to a user
of another tenant, SHALL be refused as a conflict.

#### Scenario: An existing account keeps its password

- **GIVEN** a user of this tenant whose membership was removed
- **WHEN** an administrator invites their address as a member
- **THEN** they are a member again and still sign in with their old password

### Requirement: Revoking and re-issuing

Revoking SHALL stop the link at once and keep the account. Re-issuing SHALL revoke the current link
and return a new one with a fresh expiry in one transaction. An accepted invite SHALL be neither.

#### Scenario: A revoked link stops

- **WHEN** an administrator revokes a pending invite
- **THEN** its link shows the not valid message and the account remains without a password
