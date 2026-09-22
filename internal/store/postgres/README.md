# internal/store/postgres

The PostgreSQL storage engine. It satisfies `store.Store`, `store.Tx` and
`store.UnscopedTx` with the same observable behaviour as `internal/store/sqlite`.

## Driver

`github.com/jackc/pgx/v5` is driven through `database/sql` via `pgx/v5/stdlib`.
That keeps the shared migration runner, the shared tenant-scoped query builder
and the same executor shape the SQLite engine uses, so the two engines differ
only where the dialect forces them to. The native pgx connection is still
reached through `sql.Conn.Raw`, which is what `LISTEN`/`NOTIFY` needs.

## Schema

`internal/store/migrations/0001_init.sql` is authored for SQLite and is never
edited here. `schema.go` translates it on the way in, within the same numbered
migration step, so both engines record the same migration identifiers:

| Portable schema | PostgreSQL |
| --- | --- |
| `TEXT` RFC3339 timestamps (`*_at`, `*_until`) | `TIMESTAMPTZ` |
| `INTEGER` booleans | `BOOLEAN` |
| `TEXT` JSON documents | `JSONB` |
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `BIGSERIAL` |
| `BLOB` | `BYTEA` |
| `INTEGER` | `BIGINT` |
| `LIKE '%term%'` search | `tsvector` with a GIN index |

The same step then applies what only this engine has: monthly partitions on
`events` and `audit_entries`, the generated `tasks.search_tsv` column, and
row-level security.

## Row-level security

Every tenant-owned table has RLS enabled and forced, with a policy comparing
`tenant_id` to `current_setting('tix.tenant_id')`. Each transaction sets that
value with `set_config(..., true)`, so it is transaction-local and never leaks
onto the next transaction that reuses the connection.

Forcing RLS covers the table owner, but not a superuser or a role with
`BYPASSRLS`. Migration therefore creates an unprivileged `tix_app` role, grants
it the table and sequence privileges it needs, and every transaction enters it
with `SET LOCAL ROLE`. Where the login role may not create roles, the setup is
skipped and RLS still applies to every non-superuser login.

Give the application a plain login role rather than a superuser:

    CREATE ROLE tix LOGIN PASSWORD '...';
    CREATE DATABASE tix OWNER tix;

## Partitioning and retention

`events` and `audit_entries` are ranged on `occurred_at` by month, with a
default partition as a safety net. `EnsurePartitions` creates the months up to a
given instant; `DropPartitionsBefore` drops whole months, which is what makes
retention cheap here. `PruneEvents`, `PruneAudit` and `PruneDeliveries` keep the
row-by-row contract the interface defines.

## Notifications

A transaction that appended events issues `pg_notify` on the `tix_events`
channel before committing, carrying the tenant identifier. `Store.Listen`
returns a channel that wakes on the commit instead of polling.

## Deliberate differences from SQLite

- Every write is bracketed by a statement savepoint. PostgreSQL aborts a whole
  transaction on any error, where SQLite lets the caller continue after, say, a
  unique violation; the savepoint keeps callers engine-agnostic.
- `NextSeq` locks the project row before reading the highest task number, since
  concurrent writers are not serialized the way the single SQLite writer is.
- `ClaimNextTask` and `ClaimDeliveries` pick their candidates with
  `FOR UPDATE SKIP LOCKED`, so a second worker takes the next row rather than
  queueing behind the first.
- Text search matches whole lexemes rather than substrings.

## Tests

The database-backed tests skip unless `TIX_TEST_POSTGRES_DSN` is set, and each
one runs against a database of its own:

    just pg-up
    TIX_TEST_POSTGRES_DSN='postgres://tix:tix@127.0.0.1:55432/tix?sslmode=disable' \
      go test ./internal/store/postgres/ -race

Without that DSN this package covers 6.9% of its statements rather than 85.7%,
which is why the skip announces itself: the suite gates on
`testenv.PostgresDSN`, and `TestMain` reports at the end of the run that only
SQLite was exercised. `just test` collects that report for every package into
one block. The row-level security tests have a second gate, the unprivileged
`tix_app` role, because without it the login role bypasses the policies and the
tests would prove nothing. See [docs/testing.md](../../../docs/testing.md).
