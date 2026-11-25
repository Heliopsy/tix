# Development

[CONTRIBUTING.md](../CONTRIBUTING.md) covers the process: what to run before pushing, commit format,
the specs-first rule and the CLA. This page is the reference for the recipes themselves.

```sh
just build     # bin/tix
just test      # race detector, shuffled
just check     # fast pre-push gate set
just ci        # the entire CI suite locally, in containers
```

Requires Go and [just](https://just.systems/). Linters, scanners and PostgreSQL run in containers via
podman, so nothing needs installing on the host, and every CI job invokes the same `just` recipe so
local and CI results cannot drift apart.

| Recipe | Does |
| -------- | ------ |
| `just build` / `just build-all` | Build for the host, or the full release matrix |
| `just test` / `just cover` | Tests with the race detector; coverage report |
| `just lint` `just sec` `just vuln` `just trivy` | Individual gates, in the pinned toolbox image |
| `just spec` | `openspec validate --strict` |
| `just pg-up` / `just test-postgres` | PostgreSQL in a container, and the suite against it |
| `just migration NAME` | A new migration file named by the current UTC time |
| `just cover-check` | The coverage floor, the headroom above it, and what the run skipped |
| `just test-shard SHARD` | One of the shards CI runs side by side; see [testing.md](testing.md#how-ci-runs-the-suite) |
| `just ci` | Everything CI runs, locally |
| `just lib` / `just lib-android ABI` / `just lib-ios` / `just lib-macos` / `just lib-windows` | The embedding library for the host or a platform; see [embedding.md](embedding.md#building) |
| `just lib-test` / `just lib-notices GOOS GOARCH DIR` | The library's tests, including the C harness; its licence notices |

[testing.md](testing.md) covers the suite itself: what a partial run announces, the coverage floors,
and mutation testing with `just mutate`.

## Migrations

Migrations live in `internal/store/migrations/` as one portable SQL file each. SQLite runs it as written
and the PostgreSQL engine translates it on the way in, so both engines run the same files and record the
same versions. `just migration add_widgets` creates
`internal/store/migrations/20261107153000_add_widgets.sql`, named by the current UTC time.

`0001` to `0034` keep their four-digit numbers and sort before every timestamp, so existing databases
need nothing rewritten. No numbered migration can be added after `0034`; the runner refuses the file.

The runner applies every migration a database has not recorded in `schema_migrations`, in version
order. It does not stop at the highest version applied, so a migration that merges with an earlier
timestamp than one already deployed still runs.

The tests hold these rules:

- A version is unique, and a file name is either one of the old numbers or a valid UTC timestamp.
- No timestamp is more than an hour ahead of the clock running the tests. A local time instead of
  UTC fails here.
- Every migration on `origin/main` is still there under the same name, and every migration a branch
  adds sorts after the newest one on `origin/main`. A branch that falls behind a newer migration
  rebases and renames its file to a current timestamp. `TIX_MIGRATION_BASE` names a different base;
  without git or the base ref the check skips and says so.
- SQLite and PostgreSQL record exactly the versions in the directory.

Tests that rewind a database to an earlier version, to migrate it again over existing rows, call
`migrationtest.Rewind`, which derives the undo from the migration files: a created table or index is
dropped, an added column is dropped, a dropped column is added back with its earlier definition, and data
changes are left alone. A migration doing anything else carries its own undo as comment lines, which the
runner ignores:

```sql
-- rewind: DROP VIEW open_tasks
CREATE VIEW open_tasks AS SELECT id FROM tasks WHERE status <> 'done';
```

`TestEveryMigrationRewindsToTheSchemaItStartedFrom` fails on a migration it cannot undo, so a new one
never needs a test's list edited. Row-level security on PostgreSQL attaches to the migration whose
`CREATE TABLE` creates the table, read from the files as well.

A data step (`UPDATE`, `INSERT`, `DELETE`) reaches every tenant's rows on PostgreSQL too: the runner
sets `tix.migrating` for the migration's transaction and a policy on every forced table admits it, even
when the login role is neither superuser nor `BYPASSRLS`. Test a data migration against rows that
existed before it, with `migrationtest.Rewind`, rather than rows written on the new schema. See
[the engine notes](../internal/store/postgres/README.md#migrations-run-outside-the-policies-they-create).
