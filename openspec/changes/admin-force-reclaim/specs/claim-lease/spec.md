## ADDED Requirements

### Requirement: An administrator can end a live lease

An actor holding the `task:reclaim` scope SHALL be able to end the lease another actor holds on a task,
without presenting that lease's token. The task SHALL be claimable again immediately afterwards.

The operation SHALL be reachable from the command line, the HTTP API, the browser interface and the
terminal interface. It is one product operation, and a stranded claim is noticed on every surface.

A task that holds no live lease SHALL be refused as a conflict rather than reported as a successful
reclaim of nothing. That includes a task nobody has claimed and a task whose lease has already lapsed:
both are already free.

#### Scenario: An administrator frees a held task

- **WHEN** an actor holding `task:reclaim` reclaims a task another actor holds a live lease on
- **THEN** the lease is ended and the task reads as unclaimed
- **AND** another worker can claim it

#### Scenario: A task with no live lease

- **WHEN** a reclaim is attempted on a task nobody holds, or on one whose lease has lapsed
- **THEN** the request is refused as a conflict and nothing is written

#### Scenario: Every surface reaches it

- **WHEN** the operation registry is checked
- **THEN** the reclaim carries a command-line, HTTP, browser and terminal binding

### Requirement: Reclaim authority is its own scope

The reclaim SHALL require `task:reclaim` and SHALL NOT be permitted by `task:claim`. Every worker holds
`task:claim`, and a worker able to end another worker's lease can end work it knows nothing about.

A caller without the scope SHALL be refused, and the refusal SHALL name the scope that was missing, so
an operator is not left guessing which of the vocabulary to grant.

The `admin` role SHALL hold it, through the wildcard it already carries. The `member` role SHALL NOT.

#### Scenario: A member is refused

- **WHEN** an actor with the member role, which holds `task:claim`, attempts a reclaim
- **THEN** the request is refused as forbidden
- **AND** the refusal names `task:reclaim`
- **AND** the lease is unchanged

#### Scenario: An administrator is permitted

- **WHEN** an actor holding the admin role attempts a reclaim of a live lease
- **THEN** the reclaim succeeds

### Requirement: A reclaim moves the lease and nothing else

The reclaim SHALL change only the lease: the holder, the claim instant, the expiry and the token. The
task's status SHALL be unchanged.

A task in a terminal state SHALL stay in it. The workflow's `revert_on_lease_expiry` rule SHALL NOT be
consulted, although the lease sweeper consults it for a lapse: that rule describes a worker that stopped
answering, not a person who decided. Reopening finished work is a transition the administrator performs
separately, so the trail records that they chose it.

#### Scenario: A finished task stays finished

- **WHEN** a task in a terminal state holding a live lease is reclaimed
- **THEN** the lease is ended and the task is still in that terminal state

#### Scenario: A reverting state does not revert

- **WHEN** a task in a state carrying `revert_on_lease_expiry` is reclaimed
- **THEN** the task is still in that state and no transition was recorded

### Requirement: The reclaimed holder's token stops working

After a reclaim the previous holder's lease token SHALL be refused on every lease-bound write, exactly as
it is after an expiry, and the refusal SHALL be reported as lease expiry so the worker re-claims rather
than retries.

This SHALL be the existing single lease guard rather than a second judgement: a token presented for a
task the caller does not hold is a lease expiry, and there is one place that decides it.

#### Scenario: The forced-away worker tries to carry on

- **WHEN** a worker whose lease was reclaimed attempts to renew, release or transition with its token
- **THEN** each request is refused as lease-expired and the task is unchanged

### Requirement: A reclaim is legible as a force, not as an expiry

A reclaim SHALL write an audit entry under an action distinct from the lease-expiry action, and SHALL
publish an event type distinct from the lease-expiry event type. A subscriber that cannot tell them apart
reads every administrative override as an agent failure.

The record SHALL identify who performed it, whom it was taken from, when, and that it was forced. It
SHALL carry the status the task kept and the operator's reason when one was given.

A reclaim SHALL NOT write the lease-expiry evidence columns on the task. Those columns mean the holder
stopped answering, which is how a repeatedly dying agent is found; an operator's decision recorded in
that shape would read as the failure it is not.

#### Scenario: The trail distinguishes the two

- **WHEN** a lease is reclaimed
- **THEN** an audit entry is written under the reclaim action and none under the lease-expiry action
- **AND** the entry attributes it to the reclaiming actor, and its snapshots show the previous holder and the unchanged status
- **AND** a reclaim event is published and no lease-expired event is

#### Scenario: The reason travels with the record

- **WHEN** a reclaim is performed with a reason
- **THEN** the published event carries that reason

#### Scenario: The row does not claim an expiry

- **WHEN** a lease is reclaimed
- **THEN** the task carries no lease-expiry instant and no lease-expiry holder

### Requirement: A reclaim is a compare-and-swap on the holder

The reclaim SHALL be a conditional write that re-asserts the holder the caller read and that the lease is
still live. A write keyed on the task identifier alone SHALL NOT be used: it reports success over a state
the caller never saw.

Two reclaims of the same lease SHALL produce exactly one winner. A reclaim aimed at one holder SHALL NOT
land on a lease another actor has taken since.

A renewal by the rightful holder SHALL NOT defeat a reclaim. A renewal moves the expiry and leaves the
holder, so an agent renewing on a timer cannot starve an administrator out of the override.

#### Scenario: Two administrators force at once

- **WHEN** two reclaims of the same live lease are attempted concurrently
- **THEN** exactly one reports having taken the lease
- **AND** the other reports that it wrote nothing

#### Scenario: The holder changed underneath

- **WHEN** a reclaim naming one holder is applied to a task another actor now holds
- **THEN** nothing is written and the caller is told the lease is no longer the one it read

#### Scenario: A renewal does not defeat the reclaim

- **WHEN** the holder renews its lease and an administrator then reclaims the task
- **THEN** the reclaim succeeds and the renewed lease is ended
