## ADDED Requirements

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
