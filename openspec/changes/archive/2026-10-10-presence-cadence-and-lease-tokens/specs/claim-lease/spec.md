## MODIFIED Requirements

### Requirement: Stale token rejection

A request carrying a lease token that is no longer current SHALL be rejected as lease-expired, including
when the task has since been claimed by another worker, and including when the task's lease has simply
lapsed and nobody has claimed it since. A stale token SHALL NOT be able to renew, transition, release, or
write a result.

Exactly one guard SHALL implement this judgement, and every lease-bound write SHALL reach it. A second
implementation is how the two came to disagree: the one production used accepted a token offered against a
lapsed lease, and the one that refused it had no caller at all.

A request carrying no token against a task whose lease has lapsed SHALL still be accepted. That is what
makes lazy expiry authoritative, and it is a different statement from offering a token that is no longer
held.

#### Scenario: Zombie worker after re-claim

- **WHEN** a worker whose lease expired and whose task was re-claimed by another worker attempts to transition the task with its old token
- **THEN** the request is rejected as lease-expired and the new holder's claim and the task's status are unchanged

#### Scenario: Zombie worker whose task nobody re-claimed

- **WHEN** a worker whose lease expired attempts to transition the task with its old token, and no other worker has claimed it
- **THEN** the request is rejected as lease-expired and the task's status is unchanged

#### Scenario: A caller with no token against a lapsed lease proceeds

- **WHEN** a transition is attempted with no lease token on a task whose lease has lapsed
- **THEN** the transition succeeds, because the task reads as unclaimed

#### Scenario: Zombie worker writing a result

- **WHEN** a worker with a stale token attempts to release the task with a result
- **THEN** the request is rejected as lease-expired and no result artifact is written

#### Scenario: Rejection is actionable

- **WHEN** a request is rejected because the token is stale
- **THEN** the result identifies the condition as lease expiry so the worker can re-claim rather than retry with the same token

### Requirement: Lease token required for lease-bound operations

Renew, transition, and release on a claimed task SHALL require the current lease token. A request that
omits the token or supplies one that is not current SHALL be rejected.

Every refusal on this path SHALL be reported as lease expiry, because all of them state the same fact:
the caller does not hold this lease, and the action it can take is to claim the task again.

#### Scenario: Correct token accepted

- **WHEN** a lease holder renews, transitions, or releases using its current token
- **THEN** the operation succeeds

#### Scenario: Missing token

- **WHEN** a lease-bound operation is attempted on a claimed task without a token
- **THEN** the request is rejected as lease-expired and the task is unchanged

#### Scenario: Wrong token

- **WHEN** a lease-bound operation supplies a token that was never issued for that task
- **THEN** the request is rejected as lease-expired and the task is unchanged
