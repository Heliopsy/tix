# Task numbering after an import, and the tenant in the request log

## Why

A snapshot import writes every task under the number it had at the source, but a project's counter only moved
when a task was created without one. After importing into a fresh tenant the counter still read 0, so every
task created there took a number an imported task already held and was refused as a conflict. Two production
projects had counters of 0 against highest numbers of 95 and 868 and were repaired by hand.

Separately, the request log's `tenant` field was always empty: the record is written by a middleware outside
the one that resolves the tenant, and it read a context the tenant was never put on.

## What Changes

- Writing a task under a number of its own, as every import path does, raises the project's counter to that
  number in the same transaction, on both engines. A trashed task and a tombstone count like any other.
- Reserving the next number steps past both the counter and the highest number any task of the project holds,
  so a counter left behind cannot reissue a number.
- A data migration raises every counter below its project's highest task number. It runs on PostgreSQL under
  forced row-level security through the migration flag.
- `tix status` reports projects whose counter is behind (`work.numbering_behind`), and `tix doctor` warns
  about each one in a `task-numbering` check.
- The request record carries the identifier of the tenant the request was served in.

## Capabilities

### Modified Capabilities

- `import-export`: an import leaves each project's counter at or above its tasks.
- `task-management`: the next number is above every number held.
- `cli`: doctor reports a counter behind its tasks.
- `server`: the request record names the resolved tenant.
