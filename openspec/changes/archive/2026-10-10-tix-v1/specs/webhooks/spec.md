## ADDED Requirements

### Requirement: Endpoint registration

Each tenant SHALL be able to register webhook endpoints, each carrying a target URL, a signing secret, an event type filter, and an active flag. Endpoints SHALL belong to exactly one tenant and SHALL NOT receive events from any other tenant.

#### Scenario: Registration

- **WHEN** an administrator registers an endpoint with a target URL and an event type filter
- **THEN** the endpoint is stored for that tenant and begins receiving matching events

#### Scenario: Tenant isolation

- **WHEN** an event occurs in tenant A
- **THEN** no endpoint registered by tenant B is queued for delivery of that event

#### Scenario: Inactive endpoint

- **WHEN** an endpoint's active flag is cleared and a matching event occurs
- **THEN** no delivery is queued for that endpoint

#### Scenario: Invalid target

- **WHEN** an endpoint is registered with a target that is not a valid absolute HTTP or HTTPS URL
- **THEN** the registration is rejected as a validation error

### Requirement: Event type filter

An endpoint's event type filter SHALL support exact event type names and wildcard patterns matching a family of types. Only events matching the filter SHALL be queued for that endpoint.

#### Scenario: Exact match

- **WHEN** an endpoint filters on the task transitioned type and a task is transitioned
- **THEN** a delivery is queued for that endpoint

#### Scenario: Wildcard match

- **WHEN** an endpoint filters on a wildcard covering all task event types and a task is created
- **THEN** a delivery is queued for that endpoint

#### Scenario: Non-match

- **WHEN** an endpoint filters only on task types and a project is changed
- **THEN** no delivery is queued for that endpoint

### Requirement: Durable delivery queue

Deliveries SHALL be created durably from the event log, so that a matching event registered while no dispatching process is running still results in a delivery once one runs. A delivery SHALL NOT be lost because no process was available.

#### Scenario: No process running

- **WHEN** a mutation occurs from a process that performs no dispatching and then exits
- **THEN** a pending delivery exists for each matching endpoint

#### Scenario: Later dispatch

- **WHEN** a dispatching process starts after such a mutation
- **THEN** the pending delivery is attempted

#### Scenario: Crash mid-flight

- **WHEN** a dispatching process dies after claiming a delivery but before recording an outcome
- **THEN** the delivery becomes eligible for another process once its claim lock expires

### Requirement: Delivery request format

Each delivery SHALL be an HTTP POST of the event payload carrying headers for the event type, a unique delivery identifier, a timestamp, and a signature.

#### Scenario: Headers present

- **WHEN** a delivery is sent
- **THEN** the request carries the event type, delivery identifier, timestamp, and signature headers and a JSON body

#### Scenario: Delivery identifier stability

- **WHEN** a delivery is retried after a failure
- **THEN** the delivery identifier is unchanged so the receiver can deduplicate

### Requirement: Signature

The signature SHALL be an HMAC-SHA256 computed with the endpoint's secret over the timestamp and the request body together, so that the timestamp is covered by the signature and a captured request cannot be replayed with a different timestamp.

#### Scenario: Verifiable signature

- **WHEN** a receiver recomputes the HMAC over the timestamp header and the raw body using the shared secret
- **THEN** the result equals the signature header value

#### Scenario: Timestamp tampering

- **WHEN** a captured request is replayed with a modified timestamp header and the original body and signature
- **THEN** signature verification fails

#### Scenario: Secret never disclosed

- **WHEN** an endpoint is read through any transport
- **THEN** the response does not contain the signing secret

### Requirement: Retry schedule

Failed deliveries SHALL be retried on a documented schedule with increasing backoff and a fixed maximum number of attempts. A delivery that fails its final attempt SHALL enter a terminal failed state and SHALL NOT be retried automatically.

#### Scenario: Transient failure

- **WHEN** a delivery receives a server error response
- **THEN** it is scheduled for another attempt after the next backoff interval

#### Scenario: Exhausted attempts

- **WHEN** a delivery fails its final scheduled attempt
- **THEN** it is marked failed and no further automatic attempt is made

#### Scenario: Success

- **WHEN** a delivery receives a success response
- **THEN** it is marked delivered and is not retried

#### Scenario: Timeout

- **WHEN** a target does not respond within the configured delivery timeout
- **THEN** the attempt is recorded as failed and the retry schedule applies

### Requirement: Exclusive delivery claim

A process SHALL claim a pending delivery with a lock before attempting it, and concurrent processes SHALL NOT deliver the same attempt of the same delivery more than once.

#### Scenario: Two dispatchers

- **WHEN** two processes drain the same queue at the same time
- **THEN** each pending delivery is attempted by exactly one of them

#### Scenario: Stale lock

- **WHEN** a claim lock passes its expiry without an outcome being recorded
- **THEN** another process may claim and attempt the delivery

### Requirement: Manual redelivery

An administrator SHALL be able to trigger redelivery of a delivery that already reached a terminal state, producing a new attempt against the same endpoint with the same event payload.

#### Scenario: Redeliver a failed delivery

- **WHEN** an administrator redelivers a failed delivery
- **THEN** a new attempt is queued and its outcome is recorded

#### Scenario: Redeliver a succeeded delivery

- **WHEN** an administrator redelivers a delivery that previously succeeded
- **THEN** a new attempt is queued carrying the original event payload

#### Scenario: Authorization

- **WHEN** a caller without the webhook admin scope requests a redelivery
- **THEN** the request is rejected as unauthorized

### Requirement: Delivery log

The system SHALL retain a queryable delivery log per endpoint recording each attempt with its timestamp, response status, duration, and error where applicable, paginated like every other list.

#### Scenario: Query by endpoint

- **WHEN** an administrator lists deliveries for an endpoint
- **THEN** the attempts are returned newest first with a keyset cursor for the next page

#### Scenario: Failure detail

- **WHEN** an attempt failed
- **THEN** the log entry records the response status where one was received and the transport error otherwise

#### Scenario: No secrets in the log

- **WHEN** a delivery log entry is read
- **THEN** it contains no signing secret and no credential of the target

### Requirement: Delivery outside write transactions

A delivery attempt SHALL happen strictly after the originating transaction commits and SHALL NOT be performed inside a write transaction, so that network latency cannot hold a database lock.

#### Scenario: After commit

- **WHEN** a mutation is rolled back
- **THEN** no delivery attempt is made for it

#### Scenario: Lock not held

- **WHEN** a delivery attempt is in flight against a slow or unreachable target
- **THEN** no write transaction is open for the originating mutation and other writers are not blocked

### Requirement: Configurable drain mode

The drain behaviour SHALL be configurable as inline, server, or off. Inline SHALL perform a bounded opportunistic drain after a commit in the writing process, server SHALL leave draining to a running server, and off SHALL disable draining entirely.

#### Scenario: Inline mode

- **WHEN** drain mode is inline and a mutation commits
- **THEN** the writing process attempts a bounded number of pending deliveries before returning

#### Scenario: Server mode

- **WHEN** drain mode is server and a mutation commits in a process that is not a server
- **THEN** no delivery is attempted by that process and the delivery remains pending

#### Scenario: Off mode

- **WHEN** drain mode is off
- **THEN** deliveries accumulate as pending and none are attempted

### Requirement: Reportable backlog

When nothing is draining the queue, deliveries SHALL be delayed rather than lost, and the size and age of the pending backlog SHALL be reportable by a diagnostic command.

#### Scenario: Backlog reported

- **WHEN** deliveries are pending and a diagnostic check is run
- **THEN** the report states the number of pending deliveries and the age of the oldest one

#### Scenario: Backlog drains later

- **WHEN** a dispatching process starts after a backlog has accumulated
- **THEN** every pending delivery is eventually attempted and the reported backlog returns to zero

#### Scenario: Healthy queue

- **WHEN** no deliveries are pending
- **THEN** the report states an empty backlog
