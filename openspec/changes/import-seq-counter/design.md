# Design

## Where the counter is raised

Every task insert goes through `CreateTask` in the store, and only the snapshot importer passes a number of its
own today. Raising the counter there (`UPDATE projects SET seq_counter = ? WHERE id = ? AND seq_counter < ?`)
covers merge, replace, tombstones and any later path that writes an explicit number, without each caller
having to remember. `MoveTask`, which also writes an explicit number, does the same. The statement is portable
and only touches the row when it would move it up, so a counter never goes backwards.

## The backstop in NextSeq

Two designs were on the table for task create: refuse with a clear error when the next number collides, or
compute `next = max(counter, highest held) + 1`. The second is the safer one:

- A refusal still leaves the project unable to take new tasks until somebody repairs it by hand, which is the
  outage this fixes. The computed number lets work continue, and doctor still reports the lag.
- The counter keeps its one job, never reissuing a number a hard delete took away, because it only moves up.
- The update holds the project row lock exactly as before, so concurrent creates still serialize on it. On
  PostgreSQL a writer that waited re-reads the row and steps past the number the first one committed.
- The highest number comes from the `(project_id, seq)` unique index, one index probe per create.

The subquery counts trashed tasks as well, since a trashed task still holds its number.

## Repair migration

A portable `UPDATE projects SET seq_counter = (SELECT MAX(seq) ...) WHERE seq_counter < (SELECT MAX(seq) ...)`.
A project with no tasks compares against NULL and is left alone, as is a counter already above its tasks. On
PostgreSQL it relies on the `tix.migrating` policy every forced table carries, so it reaches every tenant as a
NOBYPASSRLS owner; a test runs it that way.

## Doctor

Doctor already reads `Status` for the second factor checks. The status report gains `work.numbering_behind`, a
list of the caller's projects whose counter is behind, and doctor turns it into a `task-numbering` check. It
warns rather than fails, since creating tasks works regardless.

## Request log tenant

`logRequest` sits outside `resolveTenant` in the middleware chain, so `core.TenantFrom` on its own request
context was always empty. It now puts a small holder on the context it passes in, and `resolveTenant` writes the
tenant it pinned into it. The field stays the tenant identifier, as it was always meant to be and as the event
reader's own log lines record it.
