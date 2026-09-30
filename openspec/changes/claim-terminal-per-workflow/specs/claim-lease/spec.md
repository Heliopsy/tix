## MODIFIED Requirements

### Requirement: Claim next

A `claim next` operation SHALL atomically select and claim the highest-priority unblocked task matching a filter over project, tags, and status. It SHALL NOT return a task whose dependencies are not all in terminal states, and SHALL NOT return a task held by a live lease. A task whose own status is a terminal state of its project's workflow SHALL NOT be eligible, whatever the filter asks for, and an explicit claim of such a task SHALL be refused as a conflict.

Whether a status is terminal SHALL be decided against the workflow governing the project of the row being judged, and SHALL NOT be decided against any wider set of state names. A dependency SHALL be judged by the workflow of the project the dependency itself belongs to, which the filter does not constrain and which may differ from the workflow governing the task waiting on it. The same state name MAY be terminal under one workflow and not under another, and each row SHALL be judged only by its own.

#### Scenario: Highest priority wins

- **WHEN** `claim next` runs against a project containing several eligible tasks
- **THEN** the returned task is the highest-priority eligible task, with ties broken deterministically

#### Scenario: Blocked tasks skipped

- **WHEN** the highest-priority candidate has an unmet dependency
- **THEN** it is not returned and the next eligible unblocked task is claimed instead

#### Scenario: Finished tasks skipped

- **WHEN** `claim next` runs against a project whose only task is in a terminal state of its workflow
- **THEN** no task is returned and the empty-queue outcome is reported

#### Scenario: Finished tasks skipped among eligible ones

- **WHEN** the highest-priority candidate is in a terminal state named by its workflow, whatever that state is called
- **THEN** it is not returned and the next eligible unfinished task is claimed instead

#### Scenario: A dependency in a state its own workflow does not call terminal

- **WHEN** a task depends on a task in another project, and that dependency's status is terminal under the waiting task's workflow but is a non-terminal state of the dependency's own workflow
- **THEN** the waiting task is treated as blocked and is not returned

#### Scenario: A dependency in a state only its own workflow calls terminal

- **WHEN** a task depends on a task in another project, and that dependency's status is a terminal state of the dependency's own workflow but is not a state the waiting task's workflow calls terminal
- **THEN** the dependency counts as satisfied and the waiting task is returned

#### Scenario: A ready task in a state terminal only elsewhere

- **WHEN** `claim next` runs with no project filter and an eligible task's status is a non-terminal state of its own project's workflow while another workflow in the tenant calls that same state name terminal
- **THEN** the task is offered, and it is offered whether or not a project filter narrowed the queue

#### Scenario: Explicit claim of a finished task

- **WHEN** a worker claims a specific task that is already in a terminal state
- **THEN** the claim fails with a conflict error and no lease is taken

#### Scenario: Filtered selection

- **WHEN** `claim next` runs with a tag and status filter
- **THEN** only tasks matching every filter term are considered

#### Scenario: Concurrent claim next

- **WHEN** several workers run `claim next` against the same queue simultaneously
- **THEN** each receives a distinct task and no task is handed to two workers

### Requirement: Expiry sweeper

A sweeper SHALL materialize expiry by clearing the claim fields of tasks whose leases have passed, recording on the task that the claim expired and which actor held it, emitting a lease-expired event for each, and reverting the task's status when the workflow's state flags call for it. The clearing and the recording SHALL happen in the same statement, so a task can never read as swept without saying that a claim expired on it.

A sweep SHALL clear a claim only while the lease it read is still the lease the row carries. A task whose lease was taken again between the sweep's reading and its writing SHALL be left untouched, SHALL NOT be counted as swept, and SHALL NOT have expiry evidence or a lease-expired event written for it. A status revert SHALL be computed from the task as it stands when the revert is written, and SHALL NOT write back any other field from the state the sweep read earlier.

#### Scenario: Sweep clears a claim

- **WHEN** the sweeper processes a task whose lease has expired
- **THEN** the task's holder, lease token, and lease expiry are cleared

#### Scenario: Sweep records that the claim expired

- **WHEN** the sweeper processes a task whose lease has expired
- **THEN** the task carries the instant the claim expired and the actor that held it

#### Scenario: Sweep emits an event

- **WHEN** the sweeper materializes an expired lease
- **THEN** a lease-expired event is emitted for that task exactly once for that lease

#### Scenario: Sweep reverts status

- **WHEN** the sweeper processes a task in a state flagged revert-on-lease-expiry
- **THEN** the task's status is reverted as the workflow declares

#### Scenario: Sweep is idempotent

- **WHEN** the sweeper runs again over already-swept tasks
- **THEN** no further change is made and no additional lease-expired event is emitted

#### Scenario: Live leases untouched

- **WHEN** the sweeper runs while leases that have not expired are held
- **THEN** those tasks are left unchanged

#### Scenario: A lease taken while the sweep was reading

- **WHEN** a worker claims a task with a fresh lease after the sweep has read that task as expired and before the sweep writes
- **THEN** the new holder's lease, token and expiry are unchanged, the new holder can still renew and release under its token, no expiry evidence naming the previous holder is written, and the sweep does not count the task as swept

#### Scenario: An edit committed while the sweep was reading

- **WHEN** a task's fields are edited after the sweep has read that task as expired and before the sweep writes its status revert
- **THEN** the edit survives the sweep and only the status revert and the cleared claim fields are written
