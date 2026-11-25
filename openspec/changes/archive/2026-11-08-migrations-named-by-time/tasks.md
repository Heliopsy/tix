# Tasks

## 1. Runner

- [x] 1.1 Parse `YYYYMMDDHHMMSS_name.sql` beside the numbered files, refuse a number after 0034
- [x] 1.2 Apply every unrecorded migration in version order on both engines
- [x] 1.3 Widen `schema_migrations.version` to `BIGINT` on PostgreSQL
- [x] 1.4 Derive the migration that introduces each table for row-level security

## 2. Checks

- [x] 2.1 Replace the gap test with the uniqueness, future, base-ordering and engine-parity checks
- [x] 2.2 `migrationtest.Rewind` and the rewind tests on both engines using it

## 3. Tooling and docs

- [x] 3.1 `just migration NAME`
- [x] 3.2 `AGENTS.md` and `docs/development.md`
