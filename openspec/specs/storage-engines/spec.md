# storage-engines Specification

## Purpose

Supports SQLite and PostgreSQL behind one store interface, keeping dialect differences contained and builds free of cgo.

## Requirements

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

### Requirement: Configurable PostgreSQL connect timeout

The wait a PostgreSQL target is given to answer its startup reachability check SHALL be a configuration
key, `database.connect_timeout`, resolved through the same five layers as every other key. Its default
SHALL be 15 seconds, which is the wait the engine applied before the key existed. A non-positive value
SHALL be refused at startup, naming the key. SQLite SHALL be unaffected by it.

#### Scenario: Default preserves the previous wait

- **WHEN** no layer sets `database.connect_timeout`
- **THEN** the startup check waits 15 seconds before reporting the database unreachable

#### Scenario: A shorter wait fails sooner

- **WHEN** `database.connect_timeout` is set below the default and the PostgreSQL target does not answer
- **THEN** the process reports the database unreachable after about the configured wait rather than after 15 seconds

#### Scenario: A longer wait keeps waiting

- **WHEN** `database.connect_timeout` is set above the default and the PostgreSQL target does not answer
- **THEN** the process is still waiting after 15 seconds

#### Scenario: A non-positive wait is refused

- **WHEN** `database.connect_timeout` is set to zero or to a negative duration
- **THEN** startup fails with an error naming `database.connect_timeout`

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

The engines SHALL NOT store different instants for the same value. Where an engine's column type is
coarser than the clock, the coarser resolution SHALL be the resolution the value is authored at, so
that what is handed to an engine is what the row holds and what a caller is told. Any remaining
precision difference SHALL be documented in the engine package rather than left for a reader to
discover from a disagreement.

A migration SHALL run with whatever access its login role has and nothing more. On PostgreSQL that
places a migration outside the row-level security policies the same sequence creates, so a step that
mutates tenant-scoped rows is not guaranteed to see them. That exposure SHALL be documented in the
engine package, naming which shipped migrations it reaches and what it would cost, for as long as the
migration runner does not account for it.

#### Scenario: Same migration count

- **WHEN** migrations are applied to a fresh SQLite database and a fresh PostgreSQL database
- **THEN** both record the same migration identifiers in the same order

#### Scenario: Engine-specific statement

- **WHEN** a migration needs a statement that applies only to PostgreSQL, such as a policy or a partition
- **THEN** it is part of that same migration step and is skipped on SQLite without changing the recorded sequence

#### Scenario: Equivalent entity set

- **WHEN** the same service operations run against each engine
- **THEN** they produce equivalent stored data and identical results apart from documented search differences

#### Scenario: An instant written to either engine comes back unchanged

- **WHEN** an instant authored at the resolution the coarser engine keeps is stored and read back
- **THEN** both engines return the instant that was written

#### Scenario: The migration runner's exposure to row-level security is written down

- **WHEN** a reader looks for whether a data migration can be trusted to see the rows it names
- **THEN** the PostgreSQL engine package states that a migration runs outside its own policies, which
  roles that affects, and which shipped migrations it reaches

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

The system SHALL detect when a SQLite database file resides on a network filesystem and SHALL refuse
to open it, because such placement corrupts the database rather than merely degrading performance.
The refusal SHALL happen when the database is opened, not only when the diagnostic command runs, so
that it cannot be bypassed by never running diagnostics. The refusal SHALL name the path and the
detected filesystem type, and SHALL say what to do instead.

An operator SHALL be able to override the refusal explicitly, by flag or by configuration key,
because a refusal with no escape hatch invites patching the binary. The override SHALL NOT be the
default, and taking it SHALL be reported as a warning by the diagnostic command.

#### Scenario: Network filesystem detected on open

- **WHEN** a SQLite database on a network filesystem is opened
- **THEN** the open fails, naming the path, the detected filesystem type, and the alternatives

#### Scenario: Network filesystem detected by the diagnostic command

- **WHEN** the diagnostic command runs against a SQLite file on a network filesystem
- **THEN** it reports the path and the filesystem type

#### Scenario: Override taken

- **WHEN** the override is set and the database is on a network filesystem
- **THEN** the database opens and the diagnostic command reports a warning that the override is active

#### Scenario: Local filesystem

- **WHEN** the database file is on a local filesystem
- **THEN** the check passes

#### Scenario: Filesystem type undeterminable

- **WHEN** the filesystem type cannot be determined
- **THEN** the database opens and the check reports a warning explaining what could not be verified
  rather than silently passing

### Requirement: Warn about a database inside a file-sync directory

The system SHALL detect when a SQLite database sits inside a directory managed by a file
synchronisation tool and SHALL warn, because such a tool replaces whole files and resolves
divergence by choosing a winner, which loses one side's writes silently and can corrupt the
database. The warning SHALL name both risks, since silent loss is the more common of the two and the
less obvious.

Detection is a heuristic, so the system SHALL warn rather than refuse. A false positive that blocked
an operator from their own tasks would be worse than the risk it prevents.

#### Scenario: Database inside a synchronised directory

- **WHEN** a database is opened from a directory carrying a sync tool's markers
- **THEN** the database opens and a warning names the tool, the risk of lost writes, and the alternatives

#### Scenario: Ordinary directory

- **WHEN** a database is opened from a directory with no such markers
- **THEN** no warning is reported

### Requirement: Report existing sync conflict files

The system SHALL detect files beside the database that a synchronisation tool created to hold a
losing version, and SHALL report them, because their presence means a divergence has already
happened and the operator is unlikely to know.

#### Scenario: Conflict file present

- **WHEN** a file matching a sync tool's conflict naming sits beside the database
- **THEN** the diagnostic command reports each such file by name

### Requirement: Text search asymmetry is documented behaviour

Text search results SHALL differ between engines: PostgreSQL SHALL provide tokenized full-text
search and SQLite SHALL provide substring matching. The system SHALL report which search mode is
active and SHALL NOT claim parity between them.

The matching model is the only difference either engine is permitted. A search value SHALL be
matched as literal text on both: no character the caller typed SHALL act as a pattern operator.
On SQLite specifically, where the substring match is a `LIKE`, `%` and `_` SHALL be escaped and the
predicate SHALL declare its escape character, exactly as the explicit title and body terms already
do.

#### Scenario: Substring matching on SQLite

- **WHEN** a text search runs on SQLite
- **THEN** results are matched by substring, so stemmed or reordered terms may not match

#### Scenario: Tokenized matching on PostgreSQL

- **WHEN** the same text search runs on PostgreSQL
- **THEN** results are matched by tokens and may include matches that substring matching would miss

#### Scenario: A wildcard character in the search value is literal

- **WHEN** a text search runs for a value containing `%` or `_`
- **THEN** only tasks whose text holds that character are returned, on either engine
- **AND** tasks that a pattern operator would have matched are not returned

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

### Requirement: Commit announcement per engine

Each engine SHALL announce a transaction that appended events, as far as that engine can reach, so an
event reader acts on the commit rather than on a timer. The announcement SHALL be optional: an engine
that cannot make one SHALL remain fully usable, and a reader given none SHALL poll.

**PostgreSQL** SHALL announce through the database, so a commit made by any process connected to the
same database reaches every reader of it. A process SHALL hold at most one listener connection however
many tenants it is reading, so the cost of the announcement does not grow with the number of tenants.
A listener connection that fails SHALL be retaken, and its absence SHALL be reported nowhere but the
log, because a reader without it is late rather than wrong.

**SQLite** SHALL announce within the writing process only. SQLite offers no cross-process
notification, and the engine SHALL NOT invent one outside the database: a commit by a separate process
against the same file SHALL be observed by the reader's poll instead. This limit SHALL be documented
where a reader of the code and a reader of the specification will each meet it, rather than left to be
discovered by measurement.

An announcement SHALL NOT block the transaction making it, and SHALL be made only after the commit is
durable, so a reader woken by it never reads a position that does not yet exist.

#### Scenario: A PostgreSQL commit from another process wakes a reader

- **WHEN** one process commits a transaction appending events and another process is reading that tenant's events
- **THEN** the reading process is woken by the commit rather than by its timer

#### Scenario: One listener connection serves many tenants

- **WHEN** a process reads the events of several tenants at once
- **THEN** it holds a single listener connection for all of them

#### Scenario: A dropped listener degrades to polling

- **WHEN** the listener connection fails
- **THEN** readers continue to deliver every event on their timer, a new listener connection is taken, and no event is lost

#### Scenario: A SQLite commit in the same process wakes a reader

- **WHEN** a transaction appending events commits in the same process as a reader of that tenant
- **THEN** the reader is woken by the commit with no database round trip

#### Scenario: A SQLite commit from another process is seen on the poll

- **WHEN** a separate process commits a transaction appending events to the same SQLite file
- **THEN** the reader observes those events within its poll interval, in order, with none missing

#### Scenario: Durability precedes the announcement

- **WHEN** a reader is woken by a commit
- **THEN** every event that commit appended is already readable

### Requirement: A page of tasks reaches its tags and dependencies by task

The schema SHALL index `task_tags` and `task_deps` by tenant and then task, and the queries that
load a page's tags and dependencies, the tag filter and the blocked filter SHALL reach those tables
through a seek on the task rather than through a walk of every tag or dependency row in the tenant.

#### Scenario: A tag filter on a tenant with many tagged tasks

- **GIVEN** a tenant whose tasks carry many tags
- **WHEN** a listing is filtered with `tag:ops`
- **THEN** each task considered is checked against its own tag rows only

#### Scenario: Loading the dependencies of a page

- **WHEN** a listing loads the dependencies of the tasks on one page
- **THEN** the dependency rows are found by each task on the page, in task and then dependency order
