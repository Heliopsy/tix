## ADDED Requirements

### Requirement: Live coverage exists before a connection is acknowledged

A server SHALL establish the live coverage floor for a tenant's event stream before it acknowledges the
connection that caused that stream to start. An event committed after the connection is registered SHALL be
delivered to that connection, either by the replay of the durable log or by the live stream, and SHALL NOT
fall between them.

#### Scenario: An event committed in the handover still arrives

- **WHEN** an event is committed after a tenant's first connection has been registered but before its
  reader has begun reading
- **THEN** the subscriber receives that event

#### Scenario: Replay and live delivery meet without a gap

- **WHEN** a subscriber resumes from a cursor and events are committed continuously across the handover
  from replay to live delivery
- **THEN** it receives every event above its cursor, in sequence order, with none missing

#### Scenario: A reader that cannot take its cursor does not hold the tenant

- **WHEN** the coverage floor for a tenant cannot be read because the database is unavailable
- **THEN** the failure is logged, no reader is recorded as running for that tenant, and the next connection
  starts a fresh one
