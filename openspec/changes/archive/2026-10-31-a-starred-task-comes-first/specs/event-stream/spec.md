## MODIFIED Requirements

### Requirement: Event taxonomy

The system SHALL define a closed set of event types covering task created, task updated, task transitioned, task claimed, task released, task lease expired, task deleted, task starred, task unstarred, comment created, artifact created, dependency changed, tag changed, project changed, workflow changed, field definition changed, webhook delivery outcome, webhook redelivery, and import completed. Each mutation SHALL emit the event type that describes it, and every type a mutation can emit SHALL be nameable in a subscription filter.

#### Scenario: Transition emits its own type

- **WHEN** a task moves from one workflow state to another
- **THEN** a task transitioned event is emitted rather than a generic task updated event

#### Scenario: Lease expiry

- **WHEN** a held lease is materialized as expired
- **THEN** a task lease expired event is emitted for that task

#### Scenario: Redelivery is its own type

- **WHEN** an operator replays a webhook delivery
- **THEN** a webhook redelivery event is emitted rather than a second webhook delivery event

#### Scenario: Closed set

- **WHEN** a consumer subscribes with a filter naming an event type outside the defined set
- **THEN** the server responds with an error identifying the unknown event type

#### Scenario: Every emitted type is subscribable by name

- **WHEN** a consumer subscribes with a filter naming any type in the defined set, including webhook redelivery
- **THEN** the subscription is accepted and receives events of that type

#### Scenario: A star emits its own type

- **WHEN** an actor stars a task and later unstars it
- **THEN** a task starred event and then a task unstarred event are emitted, each carrying the task's ref and the acting actor, and neither is emitted for a star or unstar that changed nothing
