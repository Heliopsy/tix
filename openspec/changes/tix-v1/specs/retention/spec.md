## ADDED Requirements

### Requirement: Per-tenant retention policy
The system SHALL allow a retention window to be configured per tenant, and SHALL apply each tenant's policy only to that tenant's data.

#### Scenario: Tenant policy applies to its own data
- **WHEN** one tenant configures a shorter retention window than another and pruning runs
- **THEN** only the first tenant's data is pruned to the shorter window

#### Scenario: Policy is readable
- **WHEN** a caller reads the retention policy for its tenant
- **THEN** the configured windows for events, audit entries, and webhook deliveries are returned

#### Scenario: Invalid window is rejected
- **WHEN** a caller sets a retention window to a negative or unparseable duration
- **THEN** the change is rejected and the previous policy remains in effect

### Requirement: Independent windows per data class
Retention SHALL be configurable independently for events, audit entries, and webhook deliveries. Changing one window SHALL NOT change another.

#### Scenario: Changing one window leaves others alone
- **WHEN** the event retention window is shortened
- **THEN** the audit and webhook delivery windows keep their previous values

#### Scenario: Each class is pruned to its own window
- **WHEN** pruning runs with different windows for each class
- **THEN** each class is pruned according to its own window and not to any other

### Requirement: Default windows favour the audit record
Default retention SHALL keep audit entries substantially longer than events, because audit entries are the compliance record while events are a transport buffer. Webhook deliveries SHALL default to a window no longer than the audit window.

#### Scenario: Defaults on a fresh install
- **WHEN** a tenant is created and no retention policy is configured
- **THEN** the effective audit retention window is substantially longer than the effective event retention window

#### Scenario: Defaults are documented in output
- **WHEN** a caller reads the retention policy of a tenant that has not configured one
- **THEN** the effective default windows are returned and marked as defaults

### Requirement: On-demand pruning
The system SHALL provide a command that prunes data according to the effective retention policy when invoked, without requiring a server to be running.

#### Scenario: Prune from the CLI with no server
- **WHEN** an operator runs the prune command against a local database with no server process
- **THEN** expired data is removed and a summary of what was removed is reported

#### Scenario: Prune reports counts per class
- **WHEN** a prune completes
- **THEN** the number of events, audit entries, and webhook deliveries removed is reported separately

### Requirement: Optional background pruning worker
The server SHALL be able to run pruning as a background worker on a configurable interval, and this worker SHALL be optional and disabled by configuration without affecting other server functions.

#### Scenario: Worker prunes on its interval
- **WHEN** the background pruning worker is enabled and its interval elapses
- **THEN** expired data is pruned without operator action

#### Scenario: Worker can be disabled
- **WHEN** background pruning is disabled in configuration and the server starts
- **THEN** no automatic pruning occurs and on-demand pruning still works

#### Scenario: Worker failure does not stop the server
- **WHEN** a background pruning run fails
- **THEN** the failure is reported and the server continues serving requests

### Requirement: Subscriber cursor safety
Pruning SHALL NOT delete an event whose sequence is at or below the resume cursor of any live subscriber. Such events SHALL be retained past their window until no live subscriber depends on them.

#### Scenario: Lagging subscriber protects its events
- **WHEN** a live subscriber's cursor is older than the event retention window and pruning runs
- **THEN** events at or below that cursor are retained and the subscriber can still resume without a gap

#### Scenario: Events are pruned once the subscriber advances
- **WHEN** the lagging subscriber advances past those events and pruning runs again
- **THEN** the previously protected events are removed

#### Scenario: Retained events are reported
- **WHEN** pruning retains events because of a subscriber cursor
- **THEN** the run reports how many events were retained for that reason

### Requirement: Partition-based pruning on PostgreSQL
On PostgreSQL, events and audit entries SHALL be stored in monthly partitions so that pruning a fully expired period removes a partition rather than deleting rows individually.

#### Scenario: Expired month is dropped
- **WHEN** every row in a monthly partition is older than the retention window and pruning runs on PostgreSQL
- **THEN** the partition is dropped and no per-row delete is issued for it

#### Scenario: Partially expired month is not dropped
- **WHEN** a monthly partition contains rows that are still within the retention window
- **THEN** the partition is retained and only expired rows within it are eligible for removal

#### Scenario: Future partitions exist before they are needed
- **WHEN** the system writes an event whose timestamp falls in a new month
- **THEN** a partition covering that month exists and the write succeeds

### Requirement: Dry run
Pruning SHALL support a dry run that reports what would be removed without removing anything.

#### Scenario: Dry run removes nothing
- **WHEN** an operator runs pruning with the dry run option
- **THEN** counts of what would be removed are reported and the stored data is unchanged

#### Scenario: Dry run matches the real run
- **WHEN** a dry run is followed immediately by a real run with no intervening writes
- **THEN** the real run removes the counts the dry run predicted

### Requirement: Pruning emits an audit entry
A pruning run that removes data SHALL write its own audit entry recording the tenant, the classes pruned, the windows applied, and the counts removed, attributed to the system actor.

#### Scenario: Audit entry records the run
- **WHEN** pruning removes expired events
- **THEN** an audit entry exists describing the run with its counts and windows

#### Scenario: Dry run writes no audit entry
- **WHEN** pruning runs in dry run mode
- **THEN** no audit entry for a pruning run is written

#### Scenario: The pruning audit entry is not self-pruned in the same run
- **WHEN** a pruning run completes
- **THEN** the audit entry it wrote is present after the run

### Requirement: Growth reporting
The system SHALL report, per tenant, the current row counts and approximate storage sizes of events, audit entries, and webhook deliveries, together with the oldest retained timestamp in each class.

#### Scenario: Size report is available
- **WHEN** an operator requests a retention status report
- **THEN** per-class row counts, approximate sizes, and oldest retained timestamps are returned

#### Scenario: Backlog is visible
- **WHEN** undelivered webhook deliveries are accumulating
- **THEN** the report shows the pending delivery backlog

#### Scenario: Report works on both engines
- **WHEN** the report is requested against SQLite and against PostgreSQL
- **THEN** both return the same fields, with sizes reported as approximations

### Requirement: Pruning is bounded and interruptible
A pruning run SHALL operate in bounded batches so it does not hold a long write transaction, and an interrupted run SHALL leave the store consistent and SHALL be safely re-runnable.

#### Scenario: Interrupted run is consistent
- **WHEN** a pruning run is interrupted partway through
- **THEN** the store contains no partially removed record and a subsequent run completes the remaining work

#### Scenario: Pruning does not block writes indefinitely
- **WHEN** pruning runs while mutations are being written
- **THEN** mutations continue to succeed

### Requirement: Retention changes are audited and scoped
Changing a tenant's retention policy SHALL require an administrative scope and SHALL write an audit entry recording the previous and new windows.

#### Scenario: Unauthorized change is refused
- **WHEN** a caller without the administrative scope attempts to change a retention window
- **THEN** the request is refused and the policy is unchanged

#### Scenario: Change is audited
- **WHEN** an authorized caller shortens the event retention window
- **THEN** an audit entry records the previous and the new window
