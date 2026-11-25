## ADDED Requirements

### Requirement: Journal mode of an embedded database

The `embedded` package SHALL accept a journal mode of `wal` or `delete`, `wal` when none is given, and SHALL open the database in that mode; any other value SHALL fail with an invalid error before the database is opened. `TixStart` SHALL accept the same values as `journal_mode`.

#### Scenario: Delete mode

- **WHEN** a program starts the package on a database with journal mode `delete` and creates a task
- **THEN** the database reports journal mode `delete` and no `-wal` file exists beside it

#### Scenario: Unknown mode

- **WHEN** a program calls `TixStart` with `journal_mode` set to `memory`
- **THEN** it receives an error object with code `invalid` and the database is not opened

### Requirement: Checkpoint before an embedded stop

Stopping an embedded instance in WAL mode SHALL run a truncating checkpoint after the workers have stopped and before the database is closed, so every committed change is in the main database file. A checkpoint that cannot complete SHALL be reported as the stop's error and the database SHALL still be closed.

#### Scenario: WAL folded in at stop

- **WHEN** a program creates a task on an instance in WAL mode and stops it
- **THEN** the `-wal` file beside the database is empty or absent and a copy of the main file alone holds the task
