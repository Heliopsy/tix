# server-status Specification

## Purpose

Lets a running serve process record itself, so an operator can see which servers are up and what each one is serving.

## Requirements

### Requirement: A server registers itself in the store

A running `tix serve` process SHALL record itself in an installation-scoped `servers` table: once when it
starts, and again on a fixed interval for as long as it runs.

The row SHALL carry the server's identifier, its advertised listen address, its build version, the surfaces
it serves, the instant it started and the instant it was last seen. The identifier SHALL be generated with
`internal/id`, as every other identifier in the product is.

A graceful shutdown SHALL remove the row. A process that ends any other way SHALL leave it, because a
server that died is what a reader most needs to be told about.

Registration, heartbeating and removal SHALL NOT write an audit entry and SHALL NOT write an outbox event.
Both of those tables are tenant-scoped and a server belongs to no tenant, so there is no tenant to attribute
process liveness to.

#### Scenario: A started server appears

- **WHEN** a server starts against a database
- **THEN** the table holds one row for it
- **AND** the row states its address, its version, its surfaces and when it started

#### Scenario: The heartbeat moves only the last seen instant

- **WHEN** the heartbeat interval elapses
- **THEN** the row's last seen instant advances to the current time
- **AND** the instant it started is unchanged

#### Scenario: A graceful shutdown leaves nothing behind

- **WHEN** a server shuts down on a signal it handles
- **THEN** its row is gone from the table

#### Scenario: A crash leaves the row

- **WHEN** a server ends without running its shutdown
- **THEN** its row is still in the table

#### Scenario: Liveness is not audited

- **WHEN** a server registers, heartbeats and deregisters
- **THEN** no audit entry was written for any of it
- **AND** no event was written for any of it

### Requirement: The servers table is installation-scoped and carries no tenant data

The `servers` table SHALL have no `tenant_id` column, because one server serves every tenant and there is
no value the column could hold that is not a falsehood.

A server row SHALL carry no tenant identifier, no tenant key, no hostname that resolves to a tenant, no
actor identifier and no count of anything a tenant owns.

The table SHALL be reached only through `store.UnscopedTx`, and only the files the confinement requirement
names SHALL name those methods.

On PostgreSQL the table SHALL NOT have row-level security enabled and SHALL carry no tenant isolation
policy, because a predicate over a column that does not exist cannot be written, and a forced table with no
policy would be readable by nobody including the server registering itself.

#### Scenario: No tenant column exists

- **WHEN** the schema is inspected after migration
- **THEN** `servers` has no `tenant_id` column
- **AND** it is named as an exemption in the guard that requires every other table to have one

#### Scenario: No field of a row names a tenant

- **WHEN** the fields of a server row are enumerated
- **THEN** none of them carries a tenant identifier, a tenant key, a hostname or an actor

#### Scenario: PostgreSQL leaves the table unforced

- **WHEN** the PostgreSQL catalogue is asked about `servers`
- **THEN** row-level security is disabled on it
- **AND** no tenant isolation policy exists for it

#### Scenario: The unscoped door is the only door

- **WHEN** the repository is searched for the unscoped server methods
- **THEN** they are named in exactly the interface, the two engines, the registrar and the status reader
- **AND** a sixth file naming one of them fails the build

### Requirement: A reader decides staleness for itself

A reader SHALL treat a server whose last seen instant is older than the staleness threshold as not
heartbeating, computing that from the row and the current time alone.

Correctness of that judgement SHALL NOT depend on any sweeper, pruner or other background job having run.

The threshold SHALL be three heartbeat intervals, so that one missed beat is not mistaken for a dead
process.

A stale server SHALL remain in the report, marked as not heartbeating, rather than being hidden.

#### Scenario: A fresh server reads as attached

- **WHEN** a server's last seen instant is within the threshold
- **THEN** the report marks it attached

#### Scenario: A server past the threshold reads as stale

- **WHEN** a server's last seen instant is older than the threshold
- **THEN** the report marks it not heartbeating
- **AND** it says how long ago it was last seen

#### Scenario: No cleanup job is needed for the answer

- **WHEN** a server's last seen instant passes the threshold and nothing else runs at all
- **THEN** the very next read already reports it as not heartbeating

#### Scenario: A stale server is still listed

- **WHEN** the report is produced with one attached server and one stale one
- **THEN** both appear
- **AND** the two are distinguishable

### Requirement: `tix status` reports the installation

The product SHALL offer a `status` operation reporting three things: the installation, the servers, and the
work.

The installation SHALL state the build version, the schema version, the resolved store and its engine, and
the instant the report was taken.

The servers SHALL list every registered server with its identifier, address, version, surfaces, start
instant, last seen instant and whether it is attached.

The work SHALL state, for the caller's own tenant, the project count, the task count, the number of claimed
tasks, the number of leases that have expired and not been swept, and the pending and failed webhook
deliveries. It SHALL also state the number of tenants the caller can see.

The command SHALL accept `-o table`, `-o json` and `-o yaml` like every other command, and SHALL exit zero
whenever it produced the report.

The exit code SHALL NOT depend on whether any server is attached, because a server on another machine being
down is not a fault of the installation the command was run against.

#### Scenario: The report names the installation

- **WHEN** `tix status` runs against a database
- **THEN** it states the version, the schema version, the store and the engine

#### Scenario: Two attached servers are both reported

- **WHEN** two servers are registered and both heartbeating
- **THEN** the report lists two servers
- **AND** says two are attached

#### Scenario: A stale server does not fail the command

- **WHEN** the report holds a server that stopped heartbeating
- **THEN** the command exits zero
- **AND** the server is marked as not heartbeating

#### Scenario: Every output format renders the same report

- **WHEN** the command runs with `-o json` and again with `-o yaml`
- **THEN** both carry the installation, the servers and the work

### Requirement: The status report works with no server running

The status operation SHALL succeed against a local database with no server process running at all, which is
the ordinary single-user case.

With no registered servers it SHALL report an empty server list and say so in words, rather than failing or
reporting an error.

It SHALL also work against a server over HTTP, returning the same report through the API.

#### Scenario: A local database with nothing running

- **WHEN** `tix status` runs against a local database and no server has ever registered
- **THEN** it exits zero
- **AND** it says no servers are attached

#### Scenario: The same report over HTTP

- **WHEN** the command is pointed at a running server instead of a database
- **THEN** it produces a report of the same shape

### Requirement: Connection counts are attributed to the process that knows them

A server row SHALL NOT carry a connection count, because connections are held in one process's memory and a
stored count would be true only inside that process.

The report SHALL carry a connection count for a server only when the report was produced by that server
itself, and SHALL attribute it by matching the answering process's identifier to the registered row.

For every other server the count SHALL be reported as unknown, distinguishable from zero, and SHALL be
absent from the JSON rather than present and zero.

#### Scenario: The answering server reports its own connections

- **WHEN** a server produces the report and holds connections
- **THEN** the count appears against that server's row and no other

#### Scenario: Unknown is not zero

- **WHEN** a server's connection count is not known to the reader
- **THEN** the JSON omits the field
- **AND** the table output prints a dash rather than a zero

#### Scenario: A local read knows no counts

- **WHEN** the report is produced against a local database with no answering process
- **THEN** no server carries a connection count

### Requirement: The installation half and the work half are authorized separately

Reading the installation figures and the server list SHALL require the tenant administration scope, through
an action distinct from the ones that already exist.

The work figures SHALL be read within the caller's own tenant scope and SHALL report that tenant only.

The tenant count SHALL be the number of tenants the caller can already see, not the number of tenants the
installation holds, so that the report discloses nothing a reader could not already obtain.

#### Scenario: A reader without the scope is refused

- **WHEN** an actor without the tenant administration scope asks for the status
- **THEN** the request is refused

#### Scenario: The work figures are the caller's own tenant

- **WHEN** two tenants each hold tasks and one of their administrators asks for the status
- **THEN** the task count is that tenant's own
- **AND** the other tenant's tasks are not counted

#### Scenario: The tenant count matches what the reader can list

- **WHEN** the report states a tenant count
- **THEN** it equals the number of tenants the same reader gets from listing tenants

### Requirement: The JSON report is a stable shape

The JSON output SHALL carry three top-level objects: `installation`, `servers` and `work`.

`servers` SHALL be an array and SHALL be empty rather than null when there are none.

Each server SHALL carry an `attached` boolean the reader computed, so a consumer need not know the
staleness threshold, alongside the `last_seen_at` instant it was computed from.

`surfaces` SHALL be an array of strings whatever the storage shape is.

Every instant SHALL be RFC 3339 in UTC.

#### Scenario: The three objects are present

- **WHEN** the report is rendered as JSON
- **THEN** it carries `installation`, `servers` and `work`

#### Scenario: An empty list is an empty array

- **WHEN** no server is registered
- **THEN** `servers` is `[]` and not null

#### Scenario: The reader's judgement is in the document

- **WHEN** a server is stale
- **THEN** its `attached` field is false
- **AND** its `last_seen_at` is present

### Requirement: Server listings are keyset-paginated

The store listing of servers SHALL page by keyset, like every other listing in the product, and SHALL never
use `OFFSET`.

The status reader SHALL walk the pages to exhaustion under a bound, so that a pathological number of rows
cannot make the command hang.

#### Scenario: The listing pages by key

- **WHEN** servers are listed a page at a time
- **THEN** each page resumes from the previous page's cursor
- **AND** no row is repeated or skipped

#### Scenario: The reader collects every page

- **WHEN** more servers are registered than fit in one page
- **THEN** the report lists all of them
