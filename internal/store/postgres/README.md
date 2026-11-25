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
edited here. `schema.go` translates it on the way in, within the same
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

`TIMESTAMPTZ` holds microseconds, so every instant written here is truncated
to one; SQLite's RFC3339 text keeps nanoseconds. Nothing tix stores is authored
finer than that on purpose, and the one instant that used to be -- a lease
expiry, which is handed back to its holder as a field of its own as well as
written to a row -- is now cut to `lease.Precision` before either happens, so
the two engines record the same instant for one claim. Any new value with
sub-microsecond meaning has to do the same or the engines will disagree about
it.

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

### Migrations run outside the policies they create

A migration runs on the pool connection as the login role, before any
transaction has entered `tix_app` and with no `tix.tenant_id` set. On a
superuser or `BYPASSRLS` connection -- which is what the test container and
most development setups give -- the policies do not apply and a migration sees
every row. On the plain owner role recommended above they do apply, because RLS
is forced, and `current_setting('tix.tenant_id', true)` is NULL, so the
isolation predicate is never true.

So every migration transaction sets `tix.migrating` to `on` with
`set_config(..., true)` before its first statement, and every forced table
carries a second, permissive policy, `tix_migrating`, that admits every command
while that flag is on:

    USING (current_setting('tix.migrating', true) = 'on' AND current_user <> 'tix_app')

PostgreSQL ORs permissive policies, so the isolation policy is unchanged for
everything else. What keeps this from becoming a runtime path:

- The flag is transaction-local, so it ends with the migration transaction and
  never reaches the next transaction on the connection.
- Only `applyMigration` sets it. `TestOnlyTheRunnerSetsTheMigrationFlag` fails
  on any other source under `internal/` or `cmd/` that names it.
- The policy refuses `tix_app`, the role every runtime transaction enters where
  the login role may create it, so even a runtime transaction that set the flag
  would still see only its own tenant
  (`TestTheMigrationFlagWidensNoRuntimeTransaction`). Where the role cannot be
  created, the first two points are what stand.

The policy arrives with `repair_actor_links` (0.49.0) on every table that
predates it, ahead of that migration's own statements; a table created later
gets it from the migration that creates it, beside its isolation policy.
`TestEveryForcedTableAdmitsTheMigrationRunner` reads the catalogue and fails on
a forced table without it, and `TestADataStepReachesEveryTenantAsARestrictedOwner`
runs an `UPDATE`, `INSERT` and `DELETE` through the runner as a
`NOSUPERUSER NOBYPASSRLS` owner and requires them to reach both tenants.

Before 0.49.0 a data step saw no rows at all on such a role and reported
success. What that cost, migration by migration:

- `0002_project_seq` would leave every existing project's `seq_counter` at
  zero. It was unreachable: 0002 shipped in v0.1.0 alongside 0001, so no
  released database had a `projects` row when it applied.
- `0010_token_name_unique`, on an upgrade from v0.13.x or earlier, failed
  loudly: the rename affected no rows and the `CREATE UNIQUE INDEX` after it,
  which reads the heap and ignores RLS, refused the duplicates.
- `actor_user_link` (0.46.0) linked no actor to its account, so every account
  created before 0.46.0 could not sign in in the browser, was missing from
  `user ls` and lost its global admin standing.
- `actor_display_name_from_account` (0.48.0) copied no name.

`repair_actor_links` runs the last two again. The earlier migrations are not
edited and still behave as they did on a database that has not reached 0.49.0,
because the policy does not exist until then.
`TestAnUpgradeFromBefore046LinksEveryAccountUnderRowLevelSecurity` rewinds to
0034, writes the rows as 0.45 had them and migrates forward as a restricted
owner, then signs in, lists users and checks global admin standing.

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
