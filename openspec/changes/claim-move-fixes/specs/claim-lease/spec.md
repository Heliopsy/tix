## ADDED Requirements

### Requirement: A claim can start the task

A claim of one task or of the next one SHALL accept `start` and `start_status`. When either is set, the claim SHALL move the task, in the claim's transaction and under the new lease, to `start_status` or else to the first in-progress state with an edge from the task's state. A task already in that state, or in any in-progress state when none is named, SHALL stay where it is. Without either, a claim SHALL NOT move the task.

#### Scenario: Claim next and start

- **WHEN** a worker claims the next task in `todo` with start under the default workflow
- **THEN** the claim succeeds, the task is in `doing`, and the audit trail holds a claim and a transition

#### Scenario: A plain claim does not move the task

- **WHEN** a worker claims a task in `todo` without start
- **THEN** the task stays in `todo`

### Requirement: A claim that cannot start claims nothing

A claim with start whose target state the workflow lacks, or cannot reach by one edge from the task's state, or that names no state where the workflow offers no in-progress edge, SHALL be refused with the error the transition would give, and SHALL leave the task unclaimed. An unknown state SHALL be refused as invalid input naming the workflow's states.

#### Scenario: Unknown start state

- **WHEN** a worker claims a task with start_status `bogus`
- **THEN** the claim fails as invalid input listing the states and the task stays unclaimed

### Requirement: Finishing under the lease releases it

A transition into a terminal state that carries the caller's live lease token SHALL release that lease in the same transaction, recording a `task.release` audit entry and a `task.released` event after the transition's own.

#### Scenario: Move to done with the token

- **WHEN** the holder moves its claimed task to `done` with its lease token
- **THEN** the task is in `done`, unclaimed, and the audit trail shows the transition and then the release

#### Scenario: Move to a non-terminal state keeps the lease

- **WHEN** the holder moves its claimed task to `doing` with its lease token
- **THEN** the task is still claimed under the same token

### Requirement: Another holder's lease still guards a finishing move

A transition into a terminal state on a task whose live lease is held under a token the caller does not supply SHALL be refused as lease-expired, as any lease-bound write is.

#### Scenario: Somebody else holds the task

- **WHEN** an actor without the token moves a task another worker holds to `done`
- **THEN** the move is refused with lease_expired and the task keeps its status and its lease

### Requirement: Releasing a task its finishing move released is accepted

A release whose token no longer matches SHALL succeed without writing when the task is unclaimed, in a terminal state, and the release carries no status, result or comment. Any other release with a token that is not current SHALL be refused as lease-expired.

#### Scenario: Release after moving to done

- **WHEN** a holder moves its task to `done` with its token and then releases it with that token
- **THEN** the release succeeds and writes no audit entry

#### Scenario: A result after the finishing move

- **WHEN** a holder moves its task to `done` with its token and then releases it with a result
- **THEN** the release is refused with lease_expired and no artifact is written

### Requirement: The holder's tokenless move names the token

A lease-bound write refused for a missing token while the caller's own actor holds the lease SHALL still be refused as lease-expired, and its message SHALL say the caller holds the claim and name the `--lease-token` flag and the `lease_token` field of the claim's output.

#### Scenario: Holder forgets the token

- **WHEN** the actor holding a live claim moves the task without a lease token
- **THEN** the move is refused with lease_expired and the message names `--lease-token` and `lease_token`
