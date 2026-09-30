# A row is judged by its own workflow, and a sweep only clears what it still owns

## Why

Three defects in the claim path, all of them in what a worker gets handed and what the sweeper does to a
row it no longer owns.

**"Terminal" is a per-workflow property that the queue treated as a global property of a state name.**
`claim next` collected the terminal states of every workflow the filter reached into one flat list of
names and applied that list to every row it judged, including dependency rows that may live in any
project under any workflow the filter never named.

Take a tenant with project `infra` (whose workflow's terminal states are `done` and `audit`) and project
`ops` (where `done` is defined but is a waypoint, and `shipped` finishes work). Both directions are wrong:

| Situation | What happened | What should happen |
| --- | --- | --- |
| task in `infra` depends on an `ops` task sitting at `done` | handed out: `done` is in the flat list | blocked: `ops` does not call `done` finished |
| task in `infra` depends on an `ops` task sitting at `shipped` | blocked forever, reported as an empty queue | handed out |
| unscoped `claim next`, ready task in `ops` at `done` | never offered: `done` is terminal *somewhere* | offered |

The third is the one an agent meets, because the plain `tix claim next` names no project, and it is
invisible to anyone testing with a project filter.

`tix claim <ref>` was never affected: it loads the one workflow that actually governs the task.

**The sweeper could destroy a lease taken while it was reading.** Every other lease writer re-asserts its
precondition inside the `UPDATE`. `ClearClaim` was keyed on the task id and nothing else. On PostgreSQL,
transactions begin READ COMMITTED and `ExpiredLeases` is a plain `SELECT` taking no row locks, so: a
sweep reads task T as expired, a worker legitimately claims T with a fresh token and commits, and the
sweep then nulls that live lease and records the *previous* holder as the one who expired. The new holder
works on, its renew and release fail as lease-expired, its result is refused, and T reads unclaimed so a
third worker can take it. Two workers on one task and the second one's work lost.

Through the same window, the sweep's status revert rebuilt the whole row from the listing's snapshot —
title, body, status, priority, assignee, due, started, completed and custom fields, in one statement with
no version predicate — so any edit committed between the read and the write was silently reverted.

SQLite was never exposed: a write transaction takes the single writer connection and issues
`BEGIN IMMEDIATE`, so the interleave cannot occur there. The engines must still behave identically from
the service's side, so the predicate is added to both.

**Why the existing tests missed all of it.** The claim fixture had exactly one workflow, so a union of
every workflow's terminal names and the one workflow governing the row are the same set, and
`TestClaimNextUnblocksOnceDependenciesAreTerminal` passed for precisely the wrong reason. That is this
repository's recurring failure: an assertion that reads something wider than the thing it names.

## What Changes

- **The claim queue carries terminal states per workflow.** `store.ClaimNextRow.TerminalStates []string`
  becomes `Terminal []store.WorkflowTerminal`, each entry a workflow id and that workflow's terminal
  states. Both dialects render a predicate that resolves the row's project to its workflow and matches
  the status against that workflow's states only.
- **The service lists every workflow in the tenant**, not only those the filter reaches, because a
  dependency's workflow is not constrained by the filter.
- **`ClearClaim` re-asserts the lapse** it was asked to clear, and reports whether it cleared anything.
  A row that no longer carries a lapsed lease is skipped and not counted as swept; a task that is not
  there is still a not-found, which the two are distinguished by.
- **The sweep's revert is built from a fresh read** taken after the conditional clear, which is the point
  at which this transaction holds the row's write lock.
- **A project-scoped actor whose project does not exist gets a not-found**, stated in `claimScope` rather
  than falling out of a workflow lookup that no longer happens.

## Impact

- Affected specs: `claim-lease`
- Affected code: `internal/store/store.go`, `internal/store/sqlite/claim.go`,
  `internal/store/postgres/claim.go`, `internal/service/claim.go`, `internal/bench/ops.go`
- No schema change, no migration, no CLI or HTTP surface change.
