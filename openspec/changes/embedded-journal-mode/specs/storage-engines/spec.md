## MODIFIED Requirements

### Requirement: SQLite connection configuration

SQLite databases SHALL be opened in write-ahead logging mode with a configured busy timeout and
foreign key enforcement enabled, unless the embedded package asked for rollback-journal `delete` mode,
which SHALL run with full synchronous writes. These settings SHALL be applied to every connection,
including connections created after startup.

#### Scenario: Settings applied

- **WHEN** a SQLite database is opened
- **THEN** journal mode is write-ahead logging, a busy timeout is set, and foreign key enforcement is on

#### Scenario: Delete mode on request

- **WHEN** a SQLite database is opened with journal mode `delete`
- **THEN** journal mode is `delete`, synchronous is full, a busy timeout is set, and foreign key enforcement is on

#### Scenario: New connection in the pool

- **WHEN** the pool opens an additional connection during operation
- **THEN** that connection carries the same settings as the first

#### Scenario: Read-only media

- **WHEN** the database file is on a read-only filesystem and write-ahead logging cannot be established
- **THEN** the system reports the condition clearly instead of failing with an opaque driver error
