## MODIFIED Requirements

### Requirement: Migrations are forward-only

Schema changes SHALL be applied as an ordered sequence of forward-only migration files. Each file
SHALL be applied within a single transaction, and each successful application SHALL be recorded in
the `schema_migrations` table. The system SHALL NOT provide automatic down migrations.

#### Scenario: Migration applied once

- **WHEN** migrations run against a database that already recorded migration 5
- **THEN** migration 5 is skipped and every migration the database has not recorded is applied

#### Scenario: Failing migration

- **WHEN** a migration file fails partway through
- **THEN** its transaction is rolled back, no row is recorded for it, and the process reports the failure without applying later migrations

#### Scenario: Database newer than the binary

- **WHEN** the database records a migration the running binary does not contain
- **THEN** the system refuses to operate and reports the version mismatch

## ADDED Requirements

### Requirement: Migrations are named by their creation time

A migration added after 0034 SHALL be named `YYYYMMDDHHMMSS_name.sql`, its version being the UTC
time it was created. The numbered migrations 0001 to 0034 SHALL keep their versions and sort before
every timestamped one. A numbered migration after 0034 SHALL be refused.

#### Scenario: Existing database upgrades without rewriting

- **GIVEN** a database that recorded every migration up to 0034
- **WHEN** a binary with timestamped migrations opens it
- **THEN** only the timestamped migrations are applied and the recorded versions are kept as they were

#### Scenario: A numbered migration after the last one

- **WHEN** the migration set holds `0035_name.sql`
- **THEN** loading the migrations fails and names the file

### Requirement: Every unrecorded migration is applied

The runner SHALL apply every migration whose version the database has not recorded, in version
order, rather than only versions above the highest recorded one.

#### Scenario: Parallel branches merge out of timestamp order

- **GIVEN** a database that applied a migration from one branch
- **WHEN** a second branch with an earlier timestamp merges and the database migrates
- **THEN** the second branch's migration is applied
