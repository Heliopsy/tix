## ADDED Requirements

### Requirement: SQLite through a pure Go driver
SQLite support SHALL be provided by a pure Go driver so that all release builds are produced with
cgo disabled. The build SHALL fail if a dependency reintroduces a cgo requirement.

#### Scenario: Cross-compiled build
- **WHEN** the project is built for every supported platform with cgo disabled
- **THEN** every build succeeds and the resulting binaries open a SQLite database

#### Scenario: cgo dependency introduced
- **WHEN** a change adds a dependency requiring cgo
- **THEN** the build or its verification step fails and names the dependency

#### Scenario: Single static binary
- **WHEN** a released binary is run on a host with no SQLite library installed
- **THEN** it works without additional system packages

### Requirement: SQLite connection configuration
SQLite databases SHALL be opened in write-ahead logging mode with a configured busy timeout and
foreign key enforcement enabled. These settings SHALL be applied to every connection, including
connections created after startup.

#### Scenario: Settings applied
- **WHEN** a SQLite database is opened
- **THEN** journal mode is write-ahead logging, a busy timeout is set, and foreign key enforcement is on

#### Scenario: New connection in the pool
- **WHEN** the pool opens an additional connection during operation
- **THEN** that connection carries the same settings as the first

#### Scenario: Read-only media
- **WHEN** the database file is on a read-only filesystem and write-ahead logging cannot be established
- **THEN** the system reports the condition clearly instead of failing with an opaque driver error

### Requirement: Separate reader and writer pools on SQLite
The SQLite engine SHALL use a writer pool limited to one connection alongside a reader pool
permitting multiple connections. Write transactions SHALL begin in immediate mode so that lock
acquisition happens at the start of the transaction rather than at first write.

#### Scenario: Writer pool is serialized
- **WHEN** several goroutines issue writes at the same time
- **THEN** the writes are executed one at a time and all of them complete

#### Scenario: Reads proceed during a write
- **WHEN** a write transaction is in progress
- **THEN** concurrent reads continue to be served without waiting for it to commit

#### Scenario: Immediate transaction
- **WHEN** a write transaction begins
- **THEN** it acquires its write lock at the start, so a conflict surfaces before work is done rather than at commit

### Requirement: Concurrent writers serialize rather than fail
Concurrent write attempts against the same SQLite database SHALL wait and then succeed rather than
returning a database-busy error, for as long as the configured busy timeout allows. This SHALL hold
for writers in separate processes as well as separate goroutines. A timeout SHALL be reported as a
distinct, retryable error.

#### Scenario: Two processes writing
- **WHEN** two processes write to the same SQLite database at the same time
- **THEN** both writes are committed and neither reports a busy failure

#### Scenario: Busy timeout exceeded
- **WHEN** a writer cannot obtain the lock within the configured timeout
- **THEN** a retryable error naming the contention is returned rather than a generic failure

#### Scenario: No partial writes
- **WHEN** a write transaction is abandoned because of contention
- **THEN** none of its changes are visible to readers

### Requirement: PostgreSQL for shared deployments
The system SHALL support PostgreSQL as a storage engine for shared and multi-tenant deployments,
using a connection pool with configurable size and timeouts. Documentation and the diagnostic
command SHALL state that shared deployments belong on PostgreSQL.

#### Scenario: Connecting to PostgreSQL
- **WHEN** the system is configured with a PostgreSQL connection string
- **THEN** it connects, applies migrations if needed, and serves every operation the SQLite engine serves

#### Scenario: Recommendation reported
- **WHEN** the diagnostic command runs against SQLite in a configuration with more than one tenant
- **THEN** it recommends PostgreSQL and explains that SQLite offers no row-level security

#### Scenario: Pool exhaustion
- **WHEN** all pooled connections are in use and another is requested
- **THEN** the request waits up to the configured timeout and then fails with a clear resource error

### Requirement: PostgreSQL row-level security
On PostgreSQL the engine SHALL enable row-level security on tenant-owned tables and SHALL set the
per-transaction tenant setting that the policies evaluate.

#### Scenario: Policies present after migration
- **WHEN** migrations complete on PostgreSQL
- **THEN** row-level security is enabled on every tenant-owned table and the corresponding policies exist

#### Scenario: Setting scoped to the transaction
- **WHEN** a transaction ends
- **THEN** the tenant setting does not persist onto the next transaction that reuses the connection

### Requirement: PostgreSQL full-text search
On PostgreSQL the engine SHALL provide text search over task titles and descriptions using an
indexed text search vector.

#### Scenario: Indexed search
- **WHEN** a text search is performed on PostgreSQL
- **THEN** it is served through the text search index rather than a sequential scan

#### Scenario: Index maintained on write
- **WHEN** a task title or description is updated
- **THEN** the search vector is updated in the same transaction and the task is findable by its new text

### Requirement: Partitioning of high-volume tables
On PostgreSQL the `events` and `audit_entries` tables SHALL be partitioned by month so that
retention can drop whole partitions instead of deleting rows individually. Partitions for upcoming
periods SHALL be created before they are needed.

#### Scenario: Rows land in the right partition
- **WHEN** an event is written
- **THEN** it is stored in the partition for its month and is returned by ordinary queries

#### Scenario: Retention drops a partition
- **WHEN** retention removes data older than the configured window
- **THEN** whole partitions are dropped rather than rows deleted one by one

#### Scenario: Future partition exists
- **WHEN** the current month ends
- **THEN** the next month's partitions already exist and writes continue without error

### Requirement: One schema and one migration path
Both engines SHALL share a single logical schema and a single ordered migration sequence. A
migration SHALL NOT exist for one engine only; engine-specific statements SHALL live within the
same numbered migration step.

#### Scenario: Same migration count
- **WHEN** migrations are applied to a fresh SQLite database and a fresh PostgreSQL database
- **THEN** both record the same migration identifiers in the same order

#### Scenario: Engine-specific statement
- **WHEN** a migration needs a statement that applies only to PostgreSQL, such as a policy or a partition
- **THEN** it is part of that same migration step and is skipped on SQLite without changing the recorded sequence

#### Scenario: Equivalent entity set
- **WHEN** the same service operations run against each engine
- **THEN** they produce equivalent stored data and identical results apart from documented search differences

### Requirement: Dialect differences confined to engine packages
All engine-specific SQL and behaviour SHALL be contained within the per-engine packages. Parameter
placeholder rewriting SHALL be performed in one shared location so that callers write statements in
one form. Service and transport code SHALL contain no engine-specific branches.

#### Scenario: One placeholder form
- **WHEN** a statement is written in the shared builder
- **THEN** it uses a single placeholder form and is rewritten once for the active engine

#### Scenario: No engine checks outside the engine packages
- **WHEN** the codebase is inspected outside the engine packages
- **THEN** no conditional branches on the active engine are present

#### Scenario: Adding an engine
- **WHEN** support for another engine is considered
- **THEN** the work is confined to adding an engine package, with no change required in service or transport code

### Requirement: Health and readiness reporting
The system SHALL expose a health check reporting whether the database is reachable and a readiness
check reporting whether the applied migrations match the version the binary expects. Readiness
SHALL be negative when migrations are pending.

#### Scenario: Healthy and ready
- **WHEN** the database is reachable and migrations are current
- **THEN** both checks report success and name the engine in use

#### Scenario: Pending migrations
- **WHEN** the database is reachable but migrations are behind the binary
- **THEN** health reports reachable and readiness reports not ready, naming the pending migrations

#### Scenario: Database unreachable
- **WHEN** the database cannot be reached
- **THEN** health reports failure with the reason and readiness also reports not ready

#### Scenario: Database ahead of the binary
- **WHEN** the database records migrations the binary does not know
- **THEN** readiness reports not ready and states that the binary is older than the schema

### Requirement: Refuse SQLite on a network filesystem
The diagnostic command SHALL detect when a SQLite database file resides on a network filesystem and
SHALL refuse that configuration, because such placement corrupts the database rather than merely
degrading performance. The refusal SHALL name the path and the detected filesystem type.

#### Scenario: Network filesystem detected
- **WHEN** the diagnostic command runs against a SQLite file on a network filesystem
- **THEN** it fails with a non-zero exit status and reports the path and the filesystem type

#### Scenario: Local filesystem
- **WHEN** the database file is on a local filesystem
- **THEN** the check passes

#### Scenario: Filesystem type undeterminable
- **WHEN** the filesystem type cannot be determined
- **THEN** the check reports a warning explaining what could not be verified rather than silently passing

### Requirement: Text search asymmetry is documented behaviour
Text search results SHALL differ between engines: PostgreSQL SHALL provide tokenized full-text
search and SQLite SHALL provide substring matching. The system SHALL report which search mode is
active and SHALL NOT claim parity between them.

#### Scenario: Substring matching on SQLite
- **WHEN** a text search runs on SQLite
- **THEN** results are matched by substring, so stemmed or reordered terms may not match

#### Scenario: Tokenized matching on PostgreSQL
- **WHEN** the same text search runs on PostgreSQL
- **THEN** results are matched by tokens and may include matches that substring matching would miss

#### Scenario: Mode reported
- **WHEN** a caller inspects capabilities or runs the diagnostic command
- **THEN** the active search mode is reported and the difference is stated

### Requirement: Engine selection from configuration
The active engine SHALL be determined from configuration, defaulting to a local SQLite file when
nothing is specified. An unsupported or malformed connection target SHALL be reported before any
work is attempted.

#### Scenario: Default local file
- **WHEN** no database configuration is supplied
- **THEN** a local SQLite file is created at the default location, migrated, and used

#### Scenario: Explicit PostgreSQL target
- **WHEN** a PostgreSQL connection string is supplied
- **THEN** the PostgreSQL engine is selected

#### Scenario: Unrecognized target
- **WHEN** a connection target names no supported engine
- **THEN** startup fails with a message naming the supported engines
