## ADDED Requirements

### Requirement: A lease expiry that reverts a status announces the transition

When a lease expiry reverts a task's status because the workflow state it was in carries
`revert_on_lease_expiry`, the system SHALL emit a task transitioned event for that move in addition to the
task lease expired event, in the same transaction as the row it describes.

The task transitioned event SHALL carry the state the task left, the state it reached and the task's human
reference, in the same payload shape every other transition uses, so a consumer already handling
transitions needs no case of its own for this one. It SHALL additionally name the lease expiry as the
reason, so a consumer that wants to tell a sweep apart from a person's transition can.

An expiry that reverts nothing SHALL emit the task lease expired event alone. A task whose state carries
no revert rule did not move, and a transition event naming the state it is already in would have to be
filtered by every consumer.

A consumer that rebuilds task status from task transitioned events alone SHALL arrive at the status the
task actually holds after a sweep. Announcing the change only as a field inside another event type's
payload does not satisfy this: a consumer that does not read that field cannot be told by the stream that
it has missed a status change.

The expiry event SHALL keep naming the state the task was reverted to, so consumers already reading it
are not broken.

#### Scenario: A reverting expiry is visible to a transition consumer

- **WHEN** a held lease expires on a task in a state that reverts on lease expiry, and the sweeper clears it
- **THEN** a task transitioned event is emitted for that task alongside the task lease expired event
- **AND** a consumer applying only task transitioned events holds the task in the state it was reverted to

#### Scenario: The transition says where the task went and why

- **WHEN** a reverting expiry is swept
- **THEN** the task transitioned event names the state left, the state reached, the task's reference, and the lease expiry as the reason

#### Scenario: An expiry that moves nothing announces no transition

- **WHEN** a held lease expires on a task in a state that does not revert on lease expiry
- **THEN** a task lease expired event is emitted and no task transitioned event is

#### Scenario: The expiry event is unchanged for its existing readers

- **WHEN** a reverting expiry is swept
- **THEN** the task lease expired event still names the state the task was reverted to

#### Scenario: One action leaves one audit entry

- **WHEN** a reverting expiry is swept
- **THEN** the audit log holds one entry for that task under the lease expiry action and none under the transition action
