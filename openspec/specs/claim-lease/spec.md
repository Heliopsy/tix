# claim-lease Specification

## Purpose

Hands a task to exactly one worker at a time through an atomic compare-and-swap with a TTL lease, so a crashed agent's work returns to the queue instead of being lost.

## Requirements

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

Renew, transition, release, and writing an artifact on a claimed task SHALL require the current lease
token. A request that omits the token or supplies one that is not current SHALL be rejected.

Every refusal on this path SHALL be reported as lease expiry, because all of them state the same fact:
the caller does not hold this lease, and the action it can take is to claim the task again.

A comment written on a task SHALL NOT require a lease token and SHALL NOT be checked against one. A
caller holding no lease, or holding a lapsed one, SHALL therefore be able to comment on a task another
worker holds.

Which writes the lease covers SHALL be stated wherever the lease is explained to an agent, rather than
left to be inferred from the presence or absence of a flag.

#### Scenario: Correct token accepted

- **WHEN** a lease holder renews, transitions, releases, or attaches an artifact using its current token
- **THEN** the operation succeeds

#### Scenario: Missing token

- **WHEN** a lease-bound operation is attempted on a claimed task without a token
- **THEN** the request is rejected as lease-expired and the task is unchanged

#### Scenario: Wrong token

- **WHEN** a lease-bound operation supplies a token that was never issued for that task
- **THEN** the request is rejected as lease-expired and the task is unchanged

#### Scenario: An artifact write on a claimed task without a token

- **WHEN** an artifact write is attempted on a task holding a live lease and no token is supplied
- **THEN** the request is rejected as lease-expired and no artifact is written

#### Scenario: A comment is not a lease-bound operation

- **GIVEN** a task held under a live lease by one worker
- **WHEN** another caller, holding no lease on it, adds a comment
- **THEN** the comment is written and the request succeeds

#### Scenario: A result recorded through a release is lease-bound

- **GIVEN** a worker whose lease has lapsed and whose task has been re-claimed
- **WHEN** it releases the task with a comment and a result under its old token
- **THEN** the request is rejected as lease-expired and neither the comment nor the result is written

### Requirement: Stale token rejection

A request carrying a lease token that is no longer current SHALL be rejected as lease-expired, including
when the task has since been claimed by another worker, and including when the task's lease has simply
lapsed and nobody has claimed it since. A stale token SHALL NOT be able to renew, transition, release, or
write a result.

Exactly one guard SHALL implement this judgement, and every lease-bound write SHALL reach it. A second
implementation is how the two came to disagree: the one production used accepted a token offered against a
lapsed lease, and the one that refused it had no caller at all. The artifact write SHALL reach that guard
rather than carrying a judgement of its own, and SHALL NOT be the write that reaches none: a lease that
protects a task's status while leaving its results open protects nothing an agent is working for.

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

#### Scenario: Zombie worker attaching an artifact over the new holder's work

- **WHEN** a worker whose lease expired and whose task was re-claimed by another worker attempts to attach
  an artifact, whether with its old token or with none
- **THEN** the request is rejected as lease-expired and the task's artifact list is unchanged

#### Scenario: An artifact write with no token against a lapsed lease proceeds

- **WHEN** an artifact write is attempted with no lease token on a task whose lease has lapsed and which
  nobody has claimed since
- **THEN** the artifact is written, because the task reads as unclaimed

#### Scenario: An artifact write offering a token against a lapsed lease

- **WHEN** an artifact write supplies a lease token on a task that holds no live lease
- **THEN** the request is rejected as lease-expired and no artifact is written

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

### Requirement: A lease expiry is one instant, whatever the engine

A lease expiry SHALL be materialized at the coarsest resolution any supported storage engine
preserves, before it is written to a row or reported to a caller.

The expiry a claim or a renewal reports SHALL equal the expiry the task it returns carries, and SHALL
equal the expiry stored, on every engine.

Materialization SHALL truncate and SHALL NOT round. An expiry moved later than the instant it was
computed for would outlive what its holder was told, which is the same defect in the other direction.

#### Scenario: The reported expiry is the stored expiry

- **WHEN** a task is claimed, or its lease renewed, at an instant finer than the stored resolution
- **THEN** the expiry reported with the claim equals the expiry on the task returned with it
- **AND** both equal what the row holds

#### Scenario: The two engines agree about one claim

- **GIVEN** the same claim taken at the same instant against each supported engine
- **WHEN** the stored expiry is read back from each
- **THEN** both engines report the same instant

#### Scenario: Materialization never extends a lease

- **WHEN** an expiry is materialized from an instant that the stored resolution cannot represent
- **THEN** the materialized expiry is at or before that instant, never after it

### Requirement: The lease guards work product, not discussion or triage

The lease SHALL be required on the writes that produce a holder's output, and SHALL NOT be required on
the writes by which other people participate in a task somebody is working. tix is one queue shared by
humans and agents, and a guard that refused a person touching a held task would cost more than the
overwriting it prevented.

Attaching an artifact to a task SHALL be lease-bound. Artifacts are the holder's work product, and an
artifact written by a worker that no longer holds the task displaces, for every later reader, the result
of the worker that does.

Commenting on a task SHALL NOT be lease-bound, and SHALL remain available on a task holding a live lease
to a caller presenting no token. Comments are append-only discussion: no comment replaces another, so a
stale comment misleads nobody about who holds the task, and human-to-agent comment on a claimed task is
the ordinary case rather than an exception to be permitted.

Editing a task's fields SHALL NOT be lease-bound. Field edits are shared triage, and the control for two
editors colliding on a field is the task's version, which refuses an edit built on a stale read in either
direction. A lease would answer a different question and would answer it wrongly for the common case of a
human re-prioritising work an agent is doing.

The surfaces that cannot mint a lease token SHALL retain commenting and field editing on a claimed task,
and SHALL report a refused artifact write as lease expiry rather than as a failure of the surface.

#### Scenario: A human comments on a task an agent holds

- **WHEN** a caller holding no lease token comments on a task that holds another actor's live lease
- **THEN** the comment is recorded

#### Scenario: A browser session comments on a claimed task

- **WHEN** a session-authenticated browser reader, which holds no lease token, comments on a task holding
  a live lease
- **THEN** the comment is recorded and appears on the task

#### Scenario: A human re-prioritises a task an agent holds

- **WHEN** a caller holding no lease token edits the priority, assignee or title of a task holding another
  actor's live lease
- **THEN** the edit is applied

#### Scenario: A field edit built on a stale read is still refused

- **WHEN** a field edit supplies a task version that is no longer current
- **THEN** the edit is refused as a version conflict, whether or not the task holds a lease

#### Scenario: An artifact write on an unclaimed task needs nothing

- **WHEN** a caller attaches an artifact to a task that holds no lease and supplies no token
- **THEN** the artifact is written

### Requirement: The lease token travels with the artifact write on every transport

The artifact write SHALL accept the lease token on the command line, over the HTTP API, and wherever the
operation registry binds it. The token SHALL be carried as a field of the artifact input rather than as a
new operation, so the service contract is unchanged.

The command-line flag SHALL be named for the lease the way the transition's flag is, because this is a
task write that requires a lease and not an operation on the lease itself.

The documented loop SHALL NOT require an agent to handle the token by hand for the ordinary case: the exec
wrapper and release-with-result already carry results under the guard, and SHALL continue to.

#### Scenario: The token is accepted on the command line

- **WHEN** the holder of a live lease attaches an artifact from the command line passing its lease token
- **THEN** the artifact is written

#### Scenario: The token is accepted over the API

- **WHEN** the holder of a live lease attaches an artifact over the HTTP API with the token in the request body
- **THEN** the artifact is written

#### Scenario: Releasing with a result needs no separate token handling

- **WHEN** a holder releases a task supplying a structured result under its lease token
- **THEN** the result artifact is written without a second token being presented

#### Scenario: The exec wrapper is unaffected

- **WHEN** the exec wrapper claims a task, runs a child command, and releases with the exit status as a result
- **THEN** the result artifact is written and no lease token is handled outside the wrapper
