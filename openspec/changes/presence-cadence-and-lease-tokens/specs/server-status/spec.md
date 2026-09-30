## MODIFIED Requirements

### Requirement: A server registers itself in the store

A running `tix serve` process SHALL record itself in an installation-scoped `servers` table: once when it
starts, and again on a fixed interval for as long as it runs.

The row SHALL carry the server's identifier, its advertised listen address, its build version, the surfaces
it serves, the instant it started, the instant it was last seen, and the heartbeat interval this process
beats at. The identifier SHALL be generated with `internal/id`, as every other identifier in the product
is.

A graceful shutdown SHALL remove the row. A process that ends any other way SHALL leave it, because a
server that died is what a reader most needs to be told about.

A heartbeat that matches no row SHALL be reported to the caller rather than treated as success, and a
running process whose row has been removed SHALL register itself again. Leaving the loop beating against
nothing under-reports what is running, which reads as the server being down.

Registration, heartbeating and removal SHALL NOT write an audit entry and SHALL NOT write an outbox event.
Both of those tables are tenant-scoped and a server belongs to no tenant, so there is no tenant to attribute
process liveness to.

#### Scenario: A started server appears

- **WHEN** a server starts against a database
- **THEN** the table holds one row for it
- **AND** the row states its address, its version, its surfaces, when it started and how often it beats

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

#### Scenario: A beat against a row that is gone is reported

- **WHEN** a heartbeat is written for a server whose row no longer exists
- **THEN** the store reports that nothing was updated rather than reporting success

#### Scenario: A running server whose row was removed returns to the registry

- **WHEN** a running server's row is deleted underneath it and the process keeps running
- **THEN** the process registers itself again without being restarted
- **AND** the restored row carries the same identifier and the uptime the process actually has

#### Scenario: Liveness is not audited

- **WHEN** a server registers, heartbeats and deregisters
- **THEN** no audit entry was written for any of it
- **AND** no event was written for any of it

### Requirement: A reader decides staleness for itself

A reader SHALL treat a server whose last seen instant is older than the staleness threshold as not
heartbeating, computing that from the row and the current time alone.

Correctness of that judgement SHALL NOT depend on any sweeper, pruner or other background job having run.

The threshold SHALL be three heartbeat intervals, so that one missed beat is not mistaken for a dead
process. The interval SHALL be the one that server declared on its own row, not a product-wide constant,
because the cadence is set per process and only the process writing the row knows it. A row that declares
no interval SHALL be judged against the default interval, which is what every such row was judged against
before the cadence was recorded.

A stale server SHALL remain in the report, marked as not heartbeating, rather than being hidden.

#### Scenario: A fresh server reads as attached

- **WHEN** a server's last seen instant is within the threshold
- **THEN** the report marks it attached

#### Scenario: A server past the threshold reads as stale

- **WHEN** a server's last seen instant is older than the threshold
- **THEN** the report marks it not heartbeating
- **AND** it says how long ago it was last seen

#### Scenario: A server beating slower than the default is not called down between its beats

- **WHEN** a server that declares a five-minute interval was last seen six minutes ago
- **THEN** the report marks it attached

#### Scenario: A server beating faster than the default is called down promptly

- **WHEN** a server that declares a one-second interval was last seen four seconds ago
- **THEN** the report marks it not heartbeating

#### Scenario: A row that declares no interval keeps the judgement it had

- **WHEN** a row written before the cadence was recorded is judged
- **THEN** the default interval is used and the threshold is three of it

#### Scenario: No cleanup job is needed for the answer

- **WHEN** a server's last seen instant passes the threshold and nothing else runs at all
- **THEN** the very next read already reports it as not heartbeating

#### Scenario: A stale server is still listed

- **WHEN** the report is produced with one attached server and one stale one
- **THEN** both appear
- **AND** the two are distinguishable

### Requirement: The JSON report is a stable shape

The JSON output SHALL carry three top-level objects: `installation`, `servers` and `work`.

`servers` SHALL be an array and SHALL be empty rather than null when there are none.

Each server SHALL carry an `attached` boolean the reader computed, so a consumer need not know the
staleness threshold, alongside the `last_seen_at` instant it was computed from and the
`heartbeat_interval` the threshold is three of. A row that declared no interval SHALL report the default
rather than a zero, so the field is always a threshold a consumer can use.

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
- **AND** its `heartbeat_interval` is present, so the threshold that judgement used can be reconstructed
