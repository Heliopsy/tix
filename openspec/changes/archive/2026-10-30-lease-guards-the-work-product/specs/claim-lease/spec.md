## MODIFIED Requirements

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

## ADDED Requirements

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
