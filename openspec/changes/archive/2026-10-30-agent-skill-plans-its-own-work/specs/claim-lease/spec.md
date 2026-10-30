## MODIFIED Requirements

### Requirement: Lease token required for lease-bound operations

Renew, transition, and release on a claimed task SHALL require the current lease token. A request that
omits the token or supplies one that is not current SHALL be rejected.

Every refusal on this path SHALL be reported as lease expiry, because all of them state the same fact:
the caller does not hold this lease, and the action it can take is to claim the task again.

A comment written on a task SHALL NOT require a lease token and SHALL NOT be checked against one. A
caller holding no lease, or holding a lapsed one, SHALL therefore be able to comment on a task another
worker holds.

Which writes the lease covers SHALL be stated wherever the lease is explained to an agent, rather than
left to be inferred from the presence or absence of a flag.

> Superseded in part by the change `lease-guards-the-work-product`, which makes the artifact write
> lease-bound. This requirement said artifacts took no token and were never checked against one, which
> was true of the code at the time and was the defect that change fixes. The comment clause stands.

#### Scenario: Correct token accepted

- **WHEN** a lease holder renews, transitions, or releases using its current token
- **THEN** the operation succeeds

#### Scenario: Missing token

- **WHEN** a lease-bound operation is attempted on a claimed task without a token
- **THEN** the request is rejected as lease-expired and the task is unchanged

#### Scenario: Wrong token

- **WHEN** a lease-bound operation supplies a token that was never issued for that task
- **THEN** the request is rejected as lease-expired and the task is unchanged

#### Scenario: A comment is not a lease-bound operation

- **GIVEN** a task held under a live lease by one worker
- **WHEN** another caller, holding no lease on it, adds a comment
- **THEN** the comment is written and the request succeeds

#### Scenario: A result recorded through a release is lease-bound

- **GIVEN** a worker whose lease has lapsed and whose task has been re-claimed
- **WHEN** it releases the task with a comment and a result under its old token
- **THEN** the request is rejected as lease-expired and neither the comment nor the result is written

### Requirement: Lazy expiry is authoritative

Lease expiry SHALL be evaluated at read and at claim time. Every read SHALL treat a task whose lease expiry has passed as unclaimed, and correctness SHALL NOT depend on any sweeper having run.

A lapsed lease SHALL NOT by itself change the task's status. The stored status SHALL remain whatever the
departed worker left it as until a sweep applies the workflow's state flags, so freeing the task and
moving it are separate events with separate triggers.

Because of that, a selection scoped to a status SHALL NOT be presented to an agent as a safe way to drain
a queue unless a sweep is running: a crashed task sits claimable in the state it was left in, and a claim
filtered to the workflow's initial state answers that no task is available. Guidance written for agents
SHALL state this, and SHALL name the command reporting how many leases have expired and remain unswept.

#### Scenario: Read after expiry with no sweeper

- **WHEN** a task whose lease expiry has passed is read and no sweeper has run
- **THEN** the task is reported as unclaimed

#### Scenario: Claim after expiry with no sweeper

- **WHEN** a task whose lease expiry has passed is claimed and no sweeper has run
- **THEN** the claim succeeds

#### Scenario: Filtering by claimed state

- **WHEN** tasks are listed filtered to unclaimed tasks and some leases have expired without any sweeper running
- **THEN** those tasks appear in the unclaimed results

#### Scenario: Command-line-only installation

- **WHEN** an installation runs with no server and therefore no ticker-driven sweeper
- **THEN** claim, read, and queue behaviour remain correct and only the materialization of expiry is deferred

#### Scenario: A killed worker's task is claimable with nothing running

- **GIVEN** a worker holding a lease that has passed its deadline, killed without releasing
- **WHEN** another worker claims that task by reference, with no sweep having run
- **THEN** the claim succeeds

#### Scenario: The status is unchanged until a sweep

- **GIVEN** a task left in a non-initial state by a worker whose lease has lapsed
- **WHEN** the task is read before any sweep has run
- **THEN** its status is the state the departed worker left it in

#### Scenario: A status-scoped claim does not see the crashed task

- **GIVEN** a task left claimable in a non-initial state by a lapsed lease
- **WHEN** a worker claims the next task scoped to the workflow's initial state
- **THEN** no task is available, although the task is claimable

#### Scenario: A sweep moves the status back

- **GIVEN** the same task, in a state the workflow marks for reversion on lease expiry
- **WHEN** a sweep runs
- **THEN** the task is returned to the state the workflow names and is found by a status-scoped claim
