## ADDED Requirements

### Requirement: One-way import in v1
The system SHALL import and refresh data one way, from an external system into tix. It SHALL NOT write changes back to an external system in v1, and any operation that would do so SHALL be absent rather than partially implemented.

#### Scenario: Import pulls data in
- **WHEN** an operator runs an import from a configured external source
- **THEN** matching external records are created or updated in tix

#### Scenario: Local changes are not pushed
- **WHEN** a task that was imported from an external system is edited in tix
- **THEN** no request is made to the external system to apply that edit

#### Scenario: No push operation is offered
- **WHEN** a caller looks for an operation that publishes tix changes to an external system
- **THEN** no such operation exists on any access path

### Requirement: Common importer interface with three adapters
The system SHALL provide at least three import adapters behind one importer interface: a generic file adapter accepting CSV and JSON, a Jira adapter, and an OpenProject adapter. All adapters SHALL be driven through the same commands, options, mapping file, dry run, and reporting.

#### Scenario: Same command for each adapter
- **WHEN** an operator runs an import naming the generic, the Jira, or the OpenProject adapter
- **THEN** the same options and the same result reporting apply in each case

#### Scenario: Adapter is named explicitly
- **WHEN** an import is requested without naming an adapter
- **THEN** the request is refused with an error listing the available adapters

### Requirement: Generic adapter requires no code
The generic adapter SHALL allow data from any external system to be imported by shaping a CSV or JSON file and supplying a mapping file, with no code written and no rebuild of the binary.

#### Scenario: Spreadsheet import
- **WHEN** an operator exports issues from an unsupported tracker to CSV and supplies a mapping file
- **THEN** the rows are imported as tasks with their mapped statuses and fields

#### Scenario: JSON import with nested fields
- **WHEN** a JSON file whose records contain nested fields is imported with a mapping that addresses those fields
- **THEN** the nested values are imported into the mapped tix fields

#### Scenario: Unmappable file is reported
- **WHEN** a file is supplied whose columns cannot satisfy the mapping's required fields
- **THEN** the import is refused with an error naming the missing fields and nothing is written

### Requirement: Declarative YAML mapping file
Mapping SHALL be expressed in a YAML file that maps external statuses onto states of a tix workflow, external issue types onto tix constructs, and external fields onto tix built-in fields or custom field definitions. The mapping SHALL be applied without code changes.

#### Scenario: Statuses map onto a workflow
- **WHEN** a mapping assigns each external status to a state of a tix workflow and an import runs
- **THEN** each imported task holds the mapped workflow state

#### Scenario: External fields become custom fields
- **WHEN** a mapping assigns an external field to a tix custom field definition
- **THEN** imported tasks carry that value in the named custom field with the definition's type

#### Scenario: Invalid mapping is refused before import
- **WHEN** a mapping names a workflow state that the target workflow does not define
- **THEN** the import is refused with an error identifying the mapping entry and nothing is written

#### Scenario: Imported data is native
- **WHEN** an imported task is read through the normal task operations
- **THEN** its status, type, and fields behave like those of a task created in tix, with no import-specific handling required

### Requirement: Dry run before any write
Every import SHALL support a dry run that reports exactly which entities would be created, which would be updated, and which would be skipped, with the reason for each skip, and SHALL write nothing.

#### Scenario: Dry run writes nothing
- **WHEN** an import is run with the dry run option
- **THEN** a per-entity plan of creations, updates, and skips is reported and no record is created or changed

#### Scenario: Skips carry reasons
- **WHEN** a dry run determines that a record will be skipped
- **THEN** the report states why, such as an unchanged external version or a failed mapping

#### Scenario: Dry run predicts the real run
- **WHEN** a dry run is immediately followed by a real import with no change at the source
- **THEN** the real import produces the creations and updates the dry run predicted

### Requirement: External reference recorded for every imported entity
Every entity created or updated by an import SHALL carry an external reference recording the source system, the external identifier, the external URL, the external version, and the time it was last synced.

#### Scenario: Reference is recorded on creation
- **WHEN** an import creates a task from an external issue
- **THEN** the task carries an external reference naming the system, external identifier, external URL, external version, and last synced time

#### Scenario: Reference is updated on refresh
- **WHEN** a subsequent import updates that task from a newer external revision
- **THEN** the external version and last synced time on the reference are updated

#### Scenario: Reference is visible
- **WHEN** an imported task is read
- **THEN** its external reference, including the link back to the source record, is available

### Requirement: Imports are idempotent
Re-running an import SHALL update records that changed at the source and leave the rest alone. It SHALL NOT create a duplicate for an entity that already carries an external reference for the same system and external identifier.

#### Scenario: Second run creates no duplicates
- **WHEN** the same import is run twice with no change at the source
- **THEN** the second run creates nothing and the entity count is unchanged

#### Scenario: Changed source record is updated in place
- **WHEN** an external issue is modified and the import is run again
- **THEN** the existing tix task is updated rather than a second task being created

#### Scenario: Unchanged records are skipped
- **WHEN** an import encounters an external record whose external version matches the stored one
- **THEN** the record is skipped and reported as skipped

### Requirement: Per-source cursors for incremental refresh
The system SHALL store a cursor per external source so that a refresh fetches only records changed since the last successful run, and SHALL allow a full refresh that ignores the cursor.

#### Scenario: Refresh fetches only changes
- **WHEN** a refresh runs after a previous successful import
- **THEN** only records changed at the source since that run are fetched

#### Scenario: Cursor advances only on success
- **WHEN** an import fails partway through
- **THEN** the stored cursor is not advanced past the records that were not successfully processed

#### Scenario: Full refresh is available
- **WHEN** an operator requests a full refresh
- **THEN** the cursor is ignored for that run and all source records are reconsidered

### Requirement: Pagination, rate limiting, and backoff
Adapters SHALL page through source results completely, SHALL respect the source system's rate limits, and SHALL retry transient failures with backoff rather than aborting the run on the first throttled response.

#### Scenario: All pages are fetched
- **WHEN** a source returns results across several pages
- **THEN** records from every page are imported

#### Scenario: Throttling is handled
- **WHEN** the source responds that the caller is rate limited
- **THEN** the adapter waits and retries rather than failing the import

#### Scenario: Persistent failure ends the run cleanly
- **WHEN** retries are exhausted against a source that keeps failing
- **THEN** the import stops with an error reporting how far it progressed, and the store is left consistent

### Requirement: Failed or interrupted imports are safely re-runnable
An import that fails or is interrupted SHALL leave the store consistent, and re-running it SHALL complete the remaining work without duplicating what was already imported.

#### Scenario: Re-run after interruption
- **WHEN** an import is interrupted after processing part of the source and is then re-run
- **THEN** the already-imported records are recognised and updated or skipped, and the remaining records are imported

#### Scenario: No partial entities remain
- **WHEN** an import is interrupted
- **THEN** no half-written entity exists

### Requirement: Unmapped fields preserved, lossy mappings reported
External fields for which the mapping defines no target SHALL be preserved in custom fields rather than discarded. Where a mapping loses information, the import SHALL report the loss.

#### Scenario: Unmapped field is preserved
- **WHEN** a source record carries a field the mapping does not name
- **THEN** the value is stored in a custom field on the imported task and is readable afterwards

#### Scenario: Lossy mapping is reported
- **WHEN** a source concept has no tix equivalent and the mapping flattens it
- **THEN** the import result reports that concept as lossy, naming the affected records

#### Scenario: Loss is visible before import
- **WHEN** a dry run is performed with a lossy mapping
- **THEN** the lossy mappings are reported before any data is written

### Requirement: Imports emit audit entries and events as a system actor
Import operations SHALL emit audit entries and outbox events for the entities they create and update, attributed to a system actor rather than to the operator who invoked the import.

#### Scenario: Imported creations are audited
- **WHEN** an import creates tasks
- **THEN** audit entries exist for those creations with the system actor and a source of system

#### Scenario: Subscribers see imported changes
- **WHEN** an import commits while a subscriber is connected
- **THEN** the subscriber receives events for the imported entities

#### Scenario: Skipped records produce no events
- **WHEN** an import skips unchanged records
- **THEN** no event is emitted for those records

### Requirement: Credentials are supplied externally and never persisted in outputs
Credentials for external systems SHALL be supplied through configuration or environment. They SHALL NOT appear in exported snapshots, audit entries, event payloads, or logs, including error output.

#### Scenario: Credentials absent from a snapshot
- **WHEN** a tenant with configured external sources is exported
- **THEN** the snapshot contains the source configuration without its credentials

#### Scenario: Credentials absent from audit entries
- **WHEN** an external source configuration is changed
- **THEN** the audit entry records the change without the credential value

#### Scenario: Credentials absent from error output
- **WHEN** an import fails because the external system rejected authentication
- **THEN** the error reports the rejection without including the credential

### Requirement: Seam for bidirectional sync
The external reference, including its external version field, and the per-source cursors SHALL be populated in v1 even though v1 reads them only for change detection, so that push and conflict detection can be added without migrating already-imported data.

#### Scenario: External version is populated
- **WHEN** a record is imported from a source that exposes a version or revision
- **THEN** that value is stored on the external reference

#### Scenario: Change detection uses the stored version
- **WHEN** a refresh compares a source record against its stored external version
- **THEN** a differing version causes an update and a matching version causes a skip

### Requirement: Imports respect tenant isolation
An import SHALL write only into the tenant it targets, and imported data SHALL NOT be readable from any other tenant.

#### Scenario: Import is scoped to its target tenant
- **WHEN** an import runs against one tenant
- **THEN** no entity is created or modified in another tenant

#### Scenario: Imported data is invisible elsewhere
- **WHEN** a caller in another tenant lists tasks
- **THEN** no imported task from the target tenant appears
