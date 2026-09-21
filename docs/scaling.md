# Scaling

One schema, one migration path, two engines. Nothing about the data model or the API changes when the engine
does.

## SQLite

The default. The DSN is `sqlite://~/.local/share/tix/tix.db`, and the file is created on first use.

The driver is pure Go, so builds stay `CGO_ENABLED=0` and the binary has no libc dependency. That is what makes
`go install` and a scratch container image work without a toolchain.

SQLite serializes writes to one at a time. For a laptop, a single agent fleet, or a team sharing one server
process, that is not the bottleneck; claims are short transactions and reads run concurrently. It stops being
comfortable when several processes write to the same file at once, which is the case a network filesystem makes
worse rather than better.

Keep the database on local disk. SQLite over NFS or SMB is a reliable way to corrupt it.

## PostgreSQL

`internal/store/postgres` implements the same `store.Store` interface with the same observable behaviour, driven
through `database/sql` via `pgx/v5/stdlib` so the shared migration runner and the shared tenant-scoped query
builder are unchanged.

What the engine adds beyond concurrency:

- **Row-level security.** Every tenant-owned table has RLS enabled and forced. See [tenancy.md](tenancy.md).
- **Real full-text search.** A generated `tasks.search_tsv` column with a GIN index, in place of SQLite's
  `LIKE '%term%'` scan. Matching is by whole lexeme rather than substring, which is the one behavioural
  difference between the engines.
- **Partitioned history.** `events` and `audit_entries` are range partitioned on `occurred_at` by month, with a
  default partition as a safety net. Retention drops whole months instead of deleting rows.
- **`LISTEN`/`NOTIFY`.** A transaction that appended events notifies on the `tix_events` channel before
  committing, so a subscriber wakes on the commit instead of polling.
- **`FOR UPDATE SKIP LOCKED`** in `ClaimNextTask`, so a second worker takes the next row rather than queueing
  behind the first. This is the difference that matters for a large agent fleet.

The migration translates the portable SQLite schema on the way in, within the same numbered migration step, so
both engines record the same migration identifiers:

| Portable schema | PostgreSQL |
| --- | --- |
| `TEXT` RFC3339 timestamps | `TIMESTAMPTZ` |
| `INTEGER` booleans | `BOOLEAN` |
| `TEXT` JSON documents | `JSONB` |
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `BIGSERIAL` |
| `BLOB` | `BYTEA` |
| `LIKE '%term%'` search | `tsvector` with a GIN index |

### Using it

Point the DSN at PostgreSQL and every command works as it does on SQLite. The schema migrates on first use:

```console
$ tix --db 'postgres://tix:tix@127.0.0.1:55432/tix?sslmode=disable' task add "on postgres"
┌───────────┬─────────────┬────────┬──────────┬──────────┬──────┬─────┬──────────────────┬─────────┐
│ REF       │ TITLE       │ STATUS │ PRIORITY │ ASSIGNEE │ TAGS │ DUE │ UPDATED          │ BLOCKED │
├───────────┼─────────────┼────────┼──────────┼──────────┼──────┼─────┼──────────────────┼─────────┤
│ default-1 │ on postgres │ todo   │ normal   │          │      │     │ 2026-09-21 00:30 │ no      │
└───────────┴─────────────┴────────┴──────────┴──────────┴──────┴─────┴──────────────────┴─────────┘
```

`tix doctor` reports the engine and the schema version it found:

```console
$ TIX_DATABASE_DSN='postgres://tix:tix@127.0.0.1:55432/tix?sslmode=disable' tix doctor
┌───────────────┬────────┬──────────────────────────────────────────────────────────────────────────────┐
│ NAME          │ STATUS │ DETAIL                                                                       │
├───────────────┼────────┼──────────────────────────────────────────────────────────────────────────────┤
│ version       │ ok     │ dev                                                                          │
│ configuration │ ok     │ no configuration file; using defaults                                        │
│ target        │ ok     │ local postgres://tix:xxxxx@127.0.0.1:55432/tix?sslmode=disable (from config) │
│ schema        │ ok     │ version 3                                                                    │
│ identity      │ ok     │ local                                                                        │
└───────────────┴────────┴──────────────────────────────────────────────────────────────────────────────┘
```

The password is redacted wherever the DSN is printed. `tix serve --db postgres://...` takes the same target, and
a context can name it once:

```sh
tix ctx add prod --db 'postgres://tix@db.internal:5432/tix?sslmode=require' --use
```

The engine's own tests run against a real database:

```sh
just pg-up
just test-postgres
```

## When to move to PostgreSQL

Move when one of these is true, not before:

- More than one process needs to write concurrently, and you would otherwise be running several `tix serve`
  instances against one file.
- A fleet of agents is claiming from the same queue fast enough that write serialization shows up in claim
  latency.
- The event or audit history is large enough that row-by-row retention pruning is expensive, and dropping a
  month's partition is the operation you want.
- Text search over task bodies has to be more than a substring scan.
- Application-layer tenant isolation is not enough for your compliance posture and you want the database to
  refuse a cross-tenant read on its own.

A single team, a single server process and a few hundred thousand tasks do not need it. Moving is a migration and
an operational dependency; make it buy you something.

## Keyset pagination

Every list is keyset paginated. There is no `OFFSET` anywhere in the codebase, by architectural rule.

`OFFSET n` makes the database walk and discard `n` rows, so page 500 costs 500 times page 1, and a row inserted
between two requests shifts every subsequent page. A keyset cursor encodes the sort key of the last row returned
and the next page asks for rows after it: constant cost per page, and stable under concurrent inserts.

```sh
tix task ls --limit 100 --cursor "$NEXT"
tix task ls --all                          # follow cursors until exhausted
```

The cost of this is that you cannot jump to page 500, only walk. For a task list that is the right trade.

Sort fields are `created_at`, `updated_at`, `priority`, `due_at`, `seq` and `title`. Sorting on an indexed column
is what keeps a cursor cheap, which is the same reasoning behind `--indexed` on custom fields.

## Retention

```sh
tix prune                 # remove history past its retention window
tix prune --dry-run
tix prune --limit 10000   # bound the work per run
```

`retention.events` (720h), `retention.audit` (8760h) and `retention.webhook_deliveries` (720h) set the windows.
`tix serve` runs the pruner hourly by default.

Retention interacts with the event stream: a subscriber resuming from a `since_seq` older than the oldest
retained event is refused rather than silently skipped. A consumer that may be offline longer than the event
window needs either a longer window or a webhook. See [api.md](api.md).

## What to measure

- Claim latency under concurrency. This is the first thing SQLite write serialization shows up in.
- Page cost as a listing deepens. It should be flat. If it is not, something is sorting on an unindexed column.
- Event lag between a mutation committing and a subscriber seeing it.
- Pruner runtime. If a run stops fitting in its interval, shorten the retention window or raise `--limit` and
  the interval together.

`internal/bench` carries seeded fixtures and benchmarks with p95 budgets.
