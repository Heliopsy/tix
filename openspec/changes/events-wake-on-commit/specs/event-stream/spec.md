## ADDED Requirements

### Requirement: A commit wakes a subscriber

A committed event SHALL reach a subscriber without waiting for the next poll, whenever the engine can
announce the commit to that subscriber. The announcement SHALL be an optimisation over the poll and
never a replacement for it: a subscriber SHALL keep an independent timer, and an announcement that is
lost, coalesced, delayed or never made SHALL cost latency until that timer fires and SHALL NOT cost an
event.

The poll interval SHALL remain the stated worst case for delivery, so the latency a deployment is
promised does not depend on a notification arriving.

Waking a subscriber SHALL NOT change the order events are delivered in, the resume guarantee a cursor
carries, or the retry behaviour of a subscriber whose reads are failing.

#### Scenario: A commit is delivered long before the poll would fire

- **WHEN** an event is committed while a subscriber is idle and the engine can announce the commit
- **THEN** the subscriber receives the event in a small fraction of the poll interval

#### Scenario: A lost announcement costs latency and not an event

- **WHEN** an event is committed and no announcement reaches the subscriber
- **THEN** the subscriber receives the event on its next poll, in order and exactly as it would have without any announcement

#### Scenario: A subscriber that is given no announcement channel behaves as before

- **WHEN** a subscriber runs against an engine or a deployment that announces nothing
- **THEN** it polls at its configured interval and delivers every event in sequence order

#### Scenario: A transaction that appended no events announces nothing

- **WHEN** a transaction that changed domain rows but appended no event commits
- **THEN** no subscriber is woken by it

#### Scenario: An announcement does not cross tenants

- **WHEN** an event is committed for one tenant while a subscriber is watching another
- **THEN** that subscriber is not woken by the commit

#### Scenario: Ordering and resume are unchanged by the wake

- **WHEN** events are committed in quick succession and a subscriber is woken by each
- **THEN** it receives them in strictly increasing sequence order, with no duplicate and no gap, and can resume from any of their sequence numbers

## MODIFIED Requirements

### Requirement: Direct database subscription

A subscriber that reads the event log over a direct database connection with no server SHALL observe only committed events and SHALL observe each one within a bounded latency after its commit.

A commit made by the same process SHALL be observed without waiting for the poll. A commit made by
another process SHALL be observed within the poll interval, and SHALL be observed without any
coordination between the two processes.

#### Scenario: Only committed events

- **WHEN** a transaction is open and uncommitted
- **THEN** a direct database subscriber observes none of its events until the transaction commits

#### Scenario: Bounded latency

- **WHEN** a mutation commits while a direct database subscriber is running
- **THEN** the subscriber observes the event within the configured maximum wake-up interval

#### Scenario: A commit in the subscriber's own process is not delayed by the poll

- **WHEN** a mutation commits in the same process as a direct database subscriber
- **THEN** the subscriber observes the event in a small fraction of the poll interval

#### Scenario: Another process writes

- **WHEN** a separate process commits a mutation to the same database
- **THEN** the direct database subscriber observes that event without any coordination between the two processes

#### Scenario: Resume is gap-free on the direct path too

- **WHEN** a direct database subscriber resumes from the sequence number of the last event it handled, after events were committed while it was not subscribed
- **THEN** it receives every matching event committed in between, in order and once each, exactly as the served path delivers them

#### Scenario: Pruned cursor on the direct path

- **WHEN** a direct database subscriber subscribes with a sequence number older than the oldest retained event
- **THEN** it is given an error indicating the cursor is no longer available, rather than being handed the oldest surviving event as though nothing were missing
