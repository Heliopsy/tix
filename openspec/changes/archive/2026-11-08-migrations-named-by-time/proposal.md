# Migrations named by time

## Why

Migrations were numbered 0001, 0002 and on, and the runner applied only versions above the
highest one a database had recorded. Two branches each adding a migration had to agree on numbers
up front and merge strictly in order, because a lower number merged second was skipped on every
database already past it. Every migration pull request also extended hand-written rewind lists in
three tests on both engines. Parallel work queued behind whichever branch held the next number.

## What Changes

- New migrations are named by the UTC time they were created, `YYYYMMDDHHMMSS_name.sql`, and
  ordered by that. `just migration NAME` writes the file.
- `0001` to `0034` keep their numbers and sort before every timestamp, so no existing database is
  rewritten. A numbered migration after `0034` is refused.
- The runner applies every migration a database has not recorded, in version order, rather than
  only those above the highest recorded version.
- On PostgreSQL `schema_migrations.version` becomes `BIGINT`, widened in place on an existing
  database, because a timestamp does not fit a 32-bit integer.
- The rule that numbers have no unlisted gap is replaced by checks that catch real mistakes: unique
  versions, no timestamp ahead of the clock beyond an hour, migrations on `origin/main` kept under
  their names, a branch's new migrations sorting after the newest on `origin/main`, and both
  engines recording exactly the versions in the directory.
- Tests rewind a database through `migrationtest.Rewind`, which derives the undo from the
  migration files, and PostgreSQL attaches row-level security to the migration that creates each
  table, read from the files instead of a hand-kept map.

## Impact

- `internal/store/migrations`, `internal/store/migrations/migrationtest`, `internal/store/postgres`,
  the rewind tests on both engines, `justfile`, `AGENTS.md`, `docs/development.md`.
