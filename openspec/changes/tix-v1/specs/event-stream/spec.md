## ADDED Requirements

### Requirement: Transactional event append

Every mutation SHALL append an event row in the same database transaction as the domain rows it changes. If the transaction rolls back, no event SHALL exist for it, and if the transaction commits, the event SHALL be durable.

#### Scenario: Commit produces an event

- **WHEN** a task is created successfully
- **THEN** exactly one task created event exists in the durable log for that task

#### Scenario: Rollback produces no event

- **WHEN** a mutation fails and its transaction rolls back
- **THEN** no event for that mutation exists in the durable log

#### Scenario: Events exist without a server

- **WHEN** a mutation is performed by a process writing directly to the database with no server running
- **THEN** the event is present in the log and is delivered to subscribers once a server is started

#### Scenario: Events survive process death

- **WHEN** the writing process exits immediately after a commit
- **THEN** the event remains in the log and is readable afterwards

### Requirement: Monotonic sequence cursor

Each event SHALL carry a sequence number that increases monotonically within a database and never repeats. The sequence number SHALL be the cursor used by every consumer to record its position.

#### Scenario: Ordering

- **WHEN** two mutations commit one after the other
- **THEN** the second event carries a strictly greater sequence number than the first

#### Scenario: Cursor resume

- **WHEN** a consumer records the sequence number of the last event it processed and later resumes from it
- **THEN** it receives every event with a greater sequence number and none it already processed

### Requirement: Event taxonomy

The system SHALL define a closed set of event types covering task created, task updated, task transitioned, task claimed, task released, task lease expired, task deleted, comment created, artifact created, dependency changed, tag changed, project changed, workflow changed, field definition changed, webhook delivery outcome, and import completed. Each mutation SHALL emit the event type that describes it.

#### Scenario: Transition emits its own type

- **WHEN** a task moves from one workflow state to another
- **THEN** a task transitioned event is emitted rather than a generic task updated event

#### Scenario: Lease expiry

- **WHEN** a held lease is materialized as expired
- **THEN** a task lease expired event is emitted for that task

#### Scenario: Closed set

- **WHEN** a consumer subscribes with a filter naming an event type outside the defined set
- **THEN** the server responds with an error identifying the unknown event type

### Requirement: Event payload shape

Every event SHALL carry a stable shape containing an event identifier, an event type, an occurrence timestamp, the tenant, the project where applicable, the subject type and subject identifier, the actor, and a type-specific payload.

#### Scenario: Required fields present

- **WHEN** any event is delivered to a subscriber
- **THEN** it contains an event identifier, type, occurrence timestamp, tenant, subject type, subject identifier, and actor

#### Scenario: Actor attribution

- **WHEN** an event results from an import
- **THEN** the actor is recorded as the system importer rather than the operator who started the import

#### Scenario: Shape stability

- **WHEN** the same event type is emitted from two different operations
- **THEN** both events carry the identical set of envelope fields

### Requirement: WebSocket endpoint

The server SHALL expose a WebSocket endpoint over which clients receive events. The client SHALL be able to send subscribe, unsubscribe, and ping messages, and the server SHALL be able to send subscribed, event, pong, and error messages.

#### Scenario: Subscribe acknowledged

- **WHEN** a client sends a subscribe message with a valid filter
- **THEN** the server replies with a subscribed message identifying the subscription before sending any event

#### Scenario: Unsubscribe

- **WHEN** a client sends an unsubscribe message for an active subscription
- **THEN** the server stops delivering events for that subscription while the connection remains open

#### Scenario: Ping and pong

- **WHEN** a client sends a ping message
- **THEN** the server replies with a pong message

#### Scenario: Malformed message

- **WHEN** a client sends a message that cannot be parsed or names an unknown message type
- **THEN** the server replies with an error message and keeps the connection open

### Requirement: Gap-free resume

A subscribe message MAY include a since sequence number. When present, the server SHALL replay every matching event with a greater sequence number from the durable log before delivering live events, in sequence order and without duplicates or gaps.

#### Scenario: Reconnect after disconnection

- **WHEN** a client reconnects and subscribes with the sequence number of the last event it received
- **THEN** it receives every matching event that occurred while it was disconnected, in order, before any newer live event

#### Scenario: Ordering across replay boundary

- **WHEN** a replay completes and live delivery begins
- **THEN** no event is delivered twice and no sequence number in the matching set is skipped

#### Scenario: Pruned history

- **WHEN** a client subscribes with a sequence number older than the oldest retained event
- **THEN** the server sends an error indicating the cursor is no longer available rather than silently skipping events

### Requirement: Subscription filters

A subscription SHALL support filtering by project and by event type, including subscribing to all projects or all event types. Only events matching the filter SHALL be delivered on that subscription.

#### Scenario: Project filter

- **WHEN** a client subscribes filtered to a single project
- **THEN** events for tasks in other projects of the same tenant are not delivered on that subscription

#### Scenario: Type filter

- **WHEN** a client subscribes filtered to task transitioned events only
- **THEN** task created events are not delivered on that subscription

#### Scenario: Multiple subscriptions on one connection

- **WHEN** a client holds two subscriptions with different filters on the same connection
- **THEN** each delivered event identifies the subscription it matched

### Requirement: Entitled fan-out

Events SHALL be delivered only to subscribers entitled to the event's tenant and holding the event subscribe scope. A subscriber SHALL NOT receive any event belonging to a tenant it is not entitled to.

#### Scenario: Cross-tenant isolation

- **WHEN** a mutation occurs in tenant A while a subscriber is connected for tenant B
- **THEN** that subscriber receives no event for the mutation

#### Scenario: Missing scope

- **WHEN** a client whose credential lacks the event subscribe scope sends a subscribe message
- **THEN** the server replies with an authorization error and delivers no events

#### Scenario: Project-restricted credential

- **WHEN** a client authenticated with a project-restricted token subscribes to all projects
- **THEN** only events for the permitted project are delivered

### Requirement: Connection lifecycle

The server SHALL send periodic pings to detect dead connections, SHALL close a connection that fails to respond within a configured timeout, and SHALL release the subscription state of a closed connection.

#### Scenario: Server ping

- **WHEN** a connection has been idle for the configured ping interval
- **THEN** the server sends a ping to that connection

#### Scenario: Unresponsive client

- **WHEN** a client does not respond to a server ping within the configured timeout
- **THEN** the server closes the connection and discards its subscriptions

#### Scenario: Slow consumer

- **WHEN** a client cannot keep up with the rate of matching events
- **THEN** the server closes that connection with an error rather than buffering without bound, and the client can resume from its last sequence number

### Requirement: Authentication at upgrade

The server SHALL resolve the tenant from the Host header and authenticate the caller at WebSocket upgrade time using the same checks the REST API applies. An unauthenticated or cross-tenant upgrade SHALL be refused before the connection is established.

#### Scenario: Unauthenticated upgrade

- **WHEN** an upgrade request arrives without a valid credential while authentication is required
- **THEN** the upgrade is refused with an unauthenticated error and no WebSocket connection is established

#### Scenario: Cross-tenant credential

- **WHEN** an upgrade request presents a token for a tenant other than the one resolved from the Host header
- **THEN** the upgrade is refused

#### Scenario: Same checks as REST

- **WHEN** a credential that is rejected by the REST API is presented at upgrade
- **THEN** the upgrade is rejected with the same error code

### Requirement: Direct database subscription

A subscriber that reads the event log over a direct database connection with no server SHALL observe only committed events and SHALL observe each one within a bounded latency after its commit.

#### Scenario: Only committed events

- **WHEN** a transaction is open and uncommitted
- **THEN** a direct database subscriber observes none of its events until the transaction commits

#### Scenario: Bounded latency

- **WHEN** a mutation commits while a direct database subscriber is running
- **THEN** the subscriber observes the event within the configured maximum wake-up interval

#### Scenario: Another process writes

- **WHEN** a separate process commits a mutation to the same database
- **THEN** the direct database subscriber observes that event without any coordination between the two processes
