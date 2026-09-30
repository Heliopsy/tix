# Design

## Judging a row by its own workflow

The predicate has to answer "does this row's status finish work?" for rows in projects the filter never
named. A flat `status IN (...)` cannot, because the answer depends on the workflow and the workflow
depends on the row.

The shape chosen is a correlated lookup on `projects`, with one `OR` term per workflow:

```sql
EXISTS (SELECT 1 FROM projects wp
        WHERE wp.id = <row>.project_id AND wp.tenant_id = tasks.tenant_id
          AND ( (wp.workflow_id = ? AND <row>.status IN (?, ?))
             OR (wp.workflow_id = ? AND <row>.status IN (?)) ))
```

`<row>` is `tasks` for the candidate's own status and `dep` inside the dependency subquery. Both dialects
render identical text; the only difference is placeholder numbering, which the shared builder already
handles.

### Alternatives rejected

**Join `projects` into the pick query.** On PostgreSQL the pick ends in `FOR UPDATE SKIP LOCKED`, and a
row lock over a join covers every table in the `FROM`. Two workers picking different tasks in the same
project would then lock the same `projects` row and skip each other's candidates. `FOR UPDATE OF tasks`
would fix that, but a correlated subquery avoids the question entirely: row locking applies to the outer
`FROM`, which stays `tasks` alone, and sublinks are not locked.

**Expand to a per-project list of terminal names.** Terminal is per workflow, and projects outnumber
workflows. Keying on `workflow_id` keeps the term count at the number of workflows in the tenant.

**Keep a flat `status NOT IN` prefilter of the names terminal in *every* workflow.** Sound (a name
terminal everywhere is terminal for whatever workflow governs the row) and it degenerates to today's
predicate when there is one workflow. Dropped: there is no index on `tasks.status` alone — the only
status index is `idx_tasks_project_status(tenant_id, project_id, status, deleted_at)`, which the
unscoped queue cannot use anyway — so the prefilter buys nothing and costs a second way to be wrong.

### Cost at 1M+ tasks

The driving access path and the ordering are unchanged: the queue still walks `tasks` under
`(priority, created_at, id)` and stops at the first row that survives the filter. What is added per
examined candidate is one primary-key lookup on `projects`, a table with one row per project, which both
planners resolve as a nested loop on the PK. Nothing here scans: the term count is the number of
workflows in the tenant, not the number of projects and not the number of tasks, and the number of rows
examined is unchanged from before. `projects` is small enough to sit in cache in any real installation.

The cost that did grow is the dependency subquery, which now resolves a project per dependency row
instead of comparing a string. It is still bounded by the dependency count of the candidate, which is
small, and it runs under `NOT EXISTS`, so it stops at the first unfinished dependency.

List queries are untouched and stay keyset-paginated with no `OFFSET`.

## The sweeper's conditional clear

`ClearClaim` gains the predicate every other lease writer already carries:

```sql
WHERE tasks.id = ? AND tasks.lease_expires_at IS NOT NULL AND tasks.lease_expires_at <= ?
```

The instant compared against is the sweep's own `At`, the same one `ExpiredLeases` was called with, so a
lease that lapsed exactly at that instant is still the sweeper's to clear and one taken since is not. A
row already cleared has a null expiry and is skipped rather than re-cleared, which keeps the sweep
idempotent.

**Zero rows is now ambiguous**, so it is resolved rather than guessed: on no match, a count on the task id
inside the same tenant scope separates "this tenant has no such task", which stays a not-found error,
from "the lease is no longer the one you read", which returns `false` with no error. The extra statement
runs only on the miss path. `ClearClaim` therefore returns `(bool, error)`, and `sweepOne` treats `false`
as skip: the task is not counted as swept, no expiry evidence is written, and no lease-expired event is
emitted for a lease that did not expire.

### Version pinning on `UpdateTask`: not added, deliberately

The obvious answer to "the revert writes back a stale snapshot" is a version predicate on `UpdateTask`.
That is not what this change does, for two reasons.

`UpdateTask` is the write path for every task mutation in the system, and optimistic concurrency already
exists a level up, in `checkVersion`, where a caller that wants it supplies a version. Making the
predicate unconditional would turn every internal rewrite into a possible conflict and change the
contract for callers that have no version to supply.

More to the point, it is not needed here. The conditional clear is an `UPDATE` on the task row, so from
the moment it succeeds this transaction holds that row's write lock; any other writer that wants the row
blocks until commit. The fresh `GetTask` taken immediately after therefore reads the current row and
nothing can change it before the revert is written. On SQLite the whole transaction is exclusive, so the
same holds trivially. The ordering — clear first, then read, then write — is what makes the pin
unnecessary, and it is load-bearing: reading before the clear would reintroduce exactly the window being
closed.

## Test shape

Every guard here has at least two workflows that disagree about `done`, because a fixture with one
workflow cannot tell "terminal under this row's workflow" apart from "terminal under some workflow" —
which is how the defect survived a green suite.

The sweeper race is driven as two real PostgreSQL transactions in two goroutines ordered by channels. The
fake clock supplies the instants and decides nothing about the schedule, because what is under test is
the transaction interleaving and a clock cannot order that.

Two service-level guards inject what SQLite cannot produce — a `ClearClaim` that reports the row was
re-taken, and an `ExpiredLeases` that returns the row as it stood before an edit committed — through thin
decorators over the test's own real SQLite database. They exercise branches whose triggering interleave
only exists on PostgreSQL; everything the sweeper writes still goes to a real store.
