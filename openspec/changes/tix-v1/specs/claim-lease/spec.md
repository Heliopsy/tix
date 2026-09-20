## ADDED Requirements

### Requirement: Atomic claim
Claiming a task SHALL be an atomic compare-and-swap that succeeds only if the task is currently unclaimed or its lease has expired. A successful claim SHALL record the claiming worker and a lease expiry time.

#### Scenario: Claim an unclaimed task
- **WHEN** a worker claims a task that is unclaimed
- **THEN** the claim succeeds and the task is returned showing the worker as holder and a lease expiry in the future

#### Scenario: Claim a task held by a live lease
- **WHEN** a worker claims a task whose lease is held by another worker and has not expired
- **THEN** the claim fails with a conflict error and the existing holder and lease expiry are unchanged

#### Scenario: Claim a task whose lease has expired
- **WHEN** a worker claims a task whose recorded lease expiry is in the past
- **THEN** the claim succeeds, the previous holder is replaced, and a new lease expiry is recorded

#### Scenario: Re-claim by the same worker
- **WHEN** the worker currently holding a live lease claims the same task again
- **THEN** the request fails with a conflict error and the worker is directed to renew instead

### Requirement: Race-free claiming without a server
Concurrent claims of the same task SHALL resolve so that exactly one succeeds, across separate processes, with no advisory locking and with no server process running.

#### Scenario: Two processes race
- **WHEN** two independent processes claim the same unclaimed task simultaneously against the same database
- **THEN** exactly one claim succeeds and the other is reported as a conflict

#### Scenario: No server running
- **WHEN** claims are made by command-line processes with no server running
- **THEN** claiming behaves identically to claiming through a running server, including conflict reporting

#### Scenario: Many workers on one queue
- **WHEN** many workers repeatedly claim from the same project concurrently
- **THEN** no task is ever held by two workers at the same time

### Requirement: Claim failure is non-blocking
A claim that cannot be granted SHALL return a conflict result immediately. The system SHALL NOT wait, queue, or block a caller until a task becomes available.

#### Scenario: Conflict returned immediately
- **WHEN** a claim is attempted on a task held by a live lease
- **THEN** the caller receives a conflict result without waiting for the lease to expire

#### Scenario: Conflict is distinguishable
- **WHEN** a claim fails because the task is already held
- **THEN** the result is distinguishable from a not-found result and from a permission error

### Requirement: Claim next
A `claim next` operation SHALL atomically select and claim the highest-priority unblocked task matching a filter over project, labels, and status. It SHALL NOT return a task whose dependencies are not all in terminal states, and SHALL NOT return a task held by a live lease.

#### Scenario: Highest priority wins
- **WHEN** `claim next` runs against a project containing several eligible tasks
- **THEN** the returned task is the highest-priority eligible task, with ties broken deterministically

#### Scenario: Blocked tasks skipped
- **WHEN** the highest-priority candidate has an unmet dependency
- **THEN** it is not returned and the next eligible unblocked task is claimed instead

#### Scenario: Filtered selection
- **WHEN** `claim next` runs with a label and status filter
- **THEN** only tasks matching every filter term are considered

#### Scenario: Concurrent claim next
- **WHEN** several workers run `claim next` against the same queue simultaneously
- **THEN** each receives a distinct task and no task is handed to two workers

### Requirement: Empty queue is not an error
When `claim next` finds no eligible task, it SHALL report that outcome distinctly from an error condition, so that a polling worker can distinguish an empty queue from a failure.

#### Scenario: Nothing available
- **WHEN** `claim next` runs and no task matches the filter or every match is blocked or held
- **THEN** a distinct empty result is returned rather than an error

#### Scenario: Distinguishable from failure
- **WHEN** a worker cannot reach the store
- **THEN** the result is an error and is distinguishable from the empty-queue result

### Requirement: Lease tokens
Each successful claim SHALL mint a fresh opaque lease token. A token SHALL NOT be predictable from the task reference and SHALL NOT be reused across claims.

#### Scenario: Token returned on claim
- **WHEN** a claim succeeds
- **THEN** an opaque lease token is returned to the claiming worker

#### Scenario: New token on re-claim
- **WHEN** a task whose lease expired is claimed again
- **THEN** a token different from the previous claim's token is minted

#### Scenario: Token not exposed to others
- **WHEN** a task is read by an actor other than the lease holder
- **THEN** the task's claim state is visible but the lease token is not returned

### Requirement: Lease token required for lease-bound operations
Renew, transition, and release on a claimed task SHALL require the current lease token. A request that omits the token or supplies one that is not current SHALL be rejected.

#### Scenario: Correct token accepted
- **WHEN** a lease holder renews, transitions, or releases using its current token
- **THEN** the operation succeeds

#### Scenario: Missing token
- **WHEN** a lease-bound operation is attempted on a claimed task without a token
- **THEN** the request is rejected and the task is unchanged

#### Scenario: Wrong token
- **WHEN** a lease-bound operation supplies a token that was never issued for that task
- **THEN** the request is rejected and the task is unchanged

### Requirement: Stale token rejection
A request carrying a lease token that is no longer current SHALL be rejected as lease-expired, including when the task has since been claimed by another worker. A stale token SHALL NOT be able to renew, transition, release, or write a result.

#### Scenario: Zombie worker after re-claim
- **WHEN** a worker whose lease expired and whose task was re-claimed by another worker attempts to transition the task with its old token
- **THEN** the request is rejected as lease-expired and the new holder's claim and the task's status are unchanged

#### Scenario: Zombie worker writing a result
- **WHEN** a worker with a stale token attempts to release the task with a result
- **THEN** the request is rejected as lease-expired and no result artifact is written

#### Scenario: Rejection is actionable
- **WHEN** a request is rejected because the token is stale
- **THEN** the result identifies the condition as lease expiry so the worker can re-claim rather than retry with the same token

### Requirement: Configurable lease TTL
The lease time-to-live SHALL be configurable globally, overridable per workflow, and overridable per claim. The most specific value supplied SHALL apply.

#### Scenario: Global default applied
- **WHEN** a task is claimed with no per-workflow and no per-claim TTL
- **THEN** the lease expiry reflects the configured global TTL

#### Scenario: Workflow override
- **WHEN** a task is claimed in a project whose workflow declares a TTL and the claim supplies none
- **THEN** the lease expiry reflects the workflow's TTL

#### Scenario: Per-claim override
- **WHEN** a claim supplies its own TTL
- **THEN** the lease expiry reflects the supplied TTL regardless of the global and workflow values

#### Scenario: Invalid TTL
- **WHEN** a claim supplies a TTL that is zero, negative, or beyond the configured maximum
- **THEN** the claim is rejected with a validation error and no lease is taken

### Requirement: Lease renewal
A lease holder SHALL be able to renew its lease, extending the expiry by the applicable TTL measured from the time of renewal. Workers are expected to renew at roughly one third of the TTL, and this cadence SHALL be documented and used by supplied tooling.

#### Scenario: Renew extends the lease
- **WHEN** a lease holder renews with its current token
- **THEN** the lease expiry is moved to the renewal time plus the applicable TTL and the token remains valid

#### Scenario: Renew after expiry
- **WHEN** a worker renews after its lease has already expired
- **THEN** the request is rejected as lease-expired and no lease is re-established

#### Scenario: Renew an unclaimed task
- **WHEN** renewal is attempted on a task that is not claimed
- **THEN** the request is rejected and the task remains unclaimed

### Requirement: Release
A lease holder SHALL be able to release a task. A release MAY carry a final status and a structured result, and a supplied final status SHALL be validated against the project's workflow.

#### Scenario: Plain release
- **WHEN** a lease holder releases a task without a final status
- **THEN** the task becomes unclaimed, its status is unchanged, and its lease token is invalidated

#### Scenario: Release with a final status
- **WHEN** a lease holder releases a task supplying a final status permitted by the workflow
- **THEN** the task becomes unclaimed and its status is set to the supplied value

#### Scenario: Release with a disallowed status
- **WHEN** a release supplies a final status for which no transition exists from the task's current state
- **THEN** the release is rejected with a validation error and the task remains claimed

#### Scenario: Release with a result
- **WHEN** a release supplies a structured result
- **THEN** the result is stored as an artifact on the task and is retrievable after release

### Requirement: Lazy expiry is authoritative
Lease expiry SHALL be evaluated at read and at claim time. Every read SHALL treat a task whose lease expiry has passed as unclaimed, and correctness SHALL NOT depend on any sweeper having run.

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

### Requirement: Expiry sweeper
A sweeper SHALL materialize expiry by clearing the claim fields of tasks whose leases have passed, emitting a lease-expired event for each, and reverting the task's status when the workflow's state flags call for it.

#### Scenario: Sweep clears a claim
- **WHEN** the sweeper processes a task whose lease has expired
- **THEN** the task's holder, lease token, and lease expiry are cleared

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

### Requirement: Sweeper invocation
The sweeper SHALL run on a ticker while a server is running, SHALL run opportunistically and time-bounded before mutating command-line operations, and SHALL be invocable on demand by an explicit command.

#### Scenario: Ticker in the server
- **WHEN** a server is running
- **THEN** the sweeper runs repeatedly at its configured interval without any external trigger

#### Scenario: Opportunistic sweep before a mutation
- **WHEN** a mutating command-line operation runs
- **THEN** a bounded sweep is attempted first and the operation proceeds regardless of whether the sweep completed

#### Scenario: Opportunistic sweep is time-bounded
- **WHEN** an opportunistic sweep reaches its time bound with work remaining
- **THEN** it stops, the remaining work is left for a later sweep, and the triggering operation is not delayed further

#### Scenario: On-demand sweep
- **WHEN** the sweep command is invoked
- **THEN** expired leases are materialized and the number of tasks swept is reported

### Requirement: Exec wrapper
An exec operation SHALL claim a task, renew its lease on a ticker for the duration of a child command, run that command, and release the task on completion with the exit status recorded as a result artifact.

#### Scenario: Successful run
- **WHEN** exec claims a task and the child command exits zero
- **THEN** the task is released and a result artifact records exit status zero

#### Scenario: Failing child command
- **WHEN** the child command exits non-zero
- **THEN** the task is released, a result artifact records the non-zero exit status, and the exec operation itself reports failure

#### Scenario: Renewal during a long run
- **WHEN** the child command runs for longer than the lease TTL
- **THEN** the lease is renewed on its ticker so the task is not treated as expired while the command is still running

#### Scenario: No task available
- **WHEN** exec is asked to claim from a queue with no eligible task
- **THEN** the child command is not started and the empty-queue outcome is reported distinctly from an error

#### Scenario: Exec interrupted
- **WHEN** the exec operation is terminated before the child command finishes
- **THEN** renewal stops, the lease is allowed to expire, and the task becomes claimable again without manual intervention
