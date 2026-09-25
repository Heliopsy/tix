## MODIFIED Requirements

### Requirement: Expiry sweeper

A sweeper SHALL materialize expiry by clearing the claim fields of tasks whose leases have passed,
recording on the task that the claim expired and which actor held it, emitting a lease-expired event for
each, and reverting the task's status when the workflow's state flags call for it. The clearing and the
recording SHALL happen in the same statement, so a task can never read as swept without saying that a
claim expired on it.

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

## ADDED Requirements

### Requirement: An expired claim leaves durable evidence

A task whose claim expired SHALL keep, on the task itself, the instant the claim expired and the actor
that held it, so that a reader can tell a task nobody ever took from a task somebody took and then
stopped answering for.

The evidence SHALL survive the sweep that clears the lease columns. Before this, the only trace of an
expired claim on a task was the populated lease columns themselves, which the sweeper nulls within one
sweep interval, so the state was observable for under a minute and in practice never seen.

The evidence SHALL be readable from the task alone, with no extra query per task in a list, so that a
keyset-paginated listing can show it at any backlog size. The lease-expired event and its audit entry
SHALL remain the full record; the task fields are the cheap read beside the task, not a replacement for
the history.

A fresh claim on the task SHALL clear the evidence, because the question it answers is whether the work
was dropped and left dropped.

Surfaces SHALL treat the evidence as current for a bounded, documented period after the expiry, so that
the mark means "recently" rather than accumulating permanently on a long-lived backlog. The stored
fields SHALL NOT be cleared when that period passes; the bound is a read-time judgement.

#### Scenario: Evidence outlives the sweep

- **WHEN** the sweeper clears an expired claim and the task is read afterwards
- **THEN** the task reads as unclaimed and still reports the instant its claim expired and the actor that
  held it

#### Scenario: Evidence is current only for a bounded period

- **WHEN** a task whose claim expired is read within the documented period
- **THEN** it reports that its claim expired recently, and after that period has passed it no longer does,
  while the recorded instant and actor are unchanged

#### Scenario: A fresh claim clears the evidence

- **WHEN** a task whose claim expired is claimed again
- **THEN** the recorded expiry instant and previous holder are cleared and the task reports a live claim

#### Scenario: A released claim leaves no expiry evidence

- **WHEN** a worker releases a live lease rather than letting it run out
- **THEN** the task records no expired claim, because nothing was dropped

#### Scenario: Reading a list shows it without a per-task lookup

- **WHEN** a page of tasks is listed
- **THEN** each task carries its own expiry evidence in the same row, with no additional query per task

### Requirement: A queue filter naming a status no workflow defines fails the claim

A claim SHALL check the status terms of its queue filter against the union of the states of every workflow
the tenant defines, before it looks for an eligible task, and SHALL fail as not found when no workflow
declares one of them.

An unmatched queue is reported as no task available, and the published guidance tells an agent to read that
as "nothing to do right now" rather than as an error. A status that can describe no task anywhere would
therefore be answered as an idle queue: the same wrong conclusion an empty listing invited, wearing a
different code. A status some workflow declares stays a legitimate filter and is answered normally, for the
reasons the listing's requirement gives.

#### Scenario: A claim filtered by an undefined status

- **WHEN** `claim next` is filtered by a status that no workflow of the tenant defines
- **THEN** it fails as not found, naming the term, rather than reporting that no task is available

#### Scenario: A claim filtered by a declared status in another case

- **WHEN** `claim next` is filtered by a status that differs from the declared state only in case
- **THEN** it claims the eligible task, rather than reporting that no task is available
