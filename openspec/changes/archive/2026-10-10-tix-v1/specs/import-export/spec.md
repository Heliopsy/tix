## ADDED Requirements

### Requirement: Tenant snapshot export

The system SHALL export a snapshot of a tenant's data in JSON or YAML, containing its projects, workflows, field definitions, tags, tasks, subtask and dependency relationships, comments, and artifact references.

#### Scenario: Full tenant export

- **WHEN** an operator exports a tenant
- **THEN** a single snapshot document is produced containing that tenant's projects, workflows, field definitions, tasks, and their relationships

#### Scenario: Format is selectable

- **WHEN** an operator requests JSON output and then YAML output for the same tenant with no intervening changes
- **THEN** both documents describe the same content

#### Scenario: Export does not modify data

- **WHEN** an export completes
- **THEN** no exported record has been changed

### Requirement: Filtered export

Export SHALL accept filters that restrict the snapshot to a subset, at minimum by project and by task filter expression, and the resulting snapshot SHALL remain self-consistent with respect to the entities it references.

#### Scenario: Single project export

- **WHEN** an operator exports one project of a tenant
- **THEN** the snapshot contains that project's tasks and the workflow and field definitions those tasks reference, and no other project's tasks

#### Scenario: Dangling references are resolved or omitted

- **WHEN** a filtered export would include a task whose dependency lies outside the filter
- **THEN** the snapshot either includes the referenced entity or records the relationship as unresolved, and never references an entity that is absent without indication

### Requirement: Stable, ordered, diffable output

Export output SHALL be deterministic: repeated exports of unchanged data SHALL produce byte-identical documents, with collections in a defined order and object keys in a defined order, so that a snapshot can be committed to version control and diffed meaningfully.

#### Scenario: Repeated export is byte-identical

- **WHEN** the same tenant is exported twice with no intervening changes
- **THEN** the two documents are byte-identical

#### Scenario: A single change produces a small diff

- **WHEN** one task's title is changed and the tenant is exported again
- **THEN** the diff against the previous snapshot is confined to that task

#### Scenario: Ordering does not depend on the storage engine

- **WHEN** equivalent data is exported from SQLite and from PostgreSQL
- **THEN** the collection ordering is the same in both documents

### Requirement: Secrets excluded from snapshots

A snapshot SHALL NOT contain password hashes, API token values or token hashes, session material, webhook signing secrets, or external system credentials. Entities that own such material SHALL be exported without it.

#### Scenario: Users export without password material

- **WHEN** a tenant containing users is exported
- **THEN** the snapshot describes the users and contains no password hash

#### Scenario: Webhook endpoints export without secrets

- **WHEN** a tenant with configured webhook endpoints is exported
- **THEN** the endpoint configuration appears without its signing secret

#### Scenario: Import of a secret-free snapshot is accepted

- **WHEN** a snapshot with no secret material is imported
- **THEN** the import succeeds and the affected entities are created without credentials, requiring credentials to be set separately

### Requirement: Snapshot format version

Every snapshot SHALL carry a format version, and import SHALL read that version to decide whether it can process the document.

#### Scenario: Version is present

- **WHEN** a snapshot is exported
- **THEN** it declares a format version

#### Scenario: Unsupported version is refused

- **WHEN** a snapshot declaring a format version the binary does not support is imported
- **THEN** the import is refused with an error naming the unsupported version and nothing is written

#### Scenario: Missing version is refused

- **WHEN** a document with no format version is imported
- **THEN** the import is refused as invalid

#### Scenario: An older supported version is still read

- **WHEN** a snapshot declaring an older format version this build still supports is imported
- **THEN** the import proceeds normally

### Requirement: Merge and replace import modes

Import SHALL accept a snapshot in merge mode, which creates missing entities and updates matching ones while leaving unreferenced existing entities intact, or in replace mode, which makes the target match the snapshot by also removing entities within the snapshot's scope that the snapshot does not contain. The mode SHALL be explicit with no destructive default.

#### Scenario: Merge leaves unrelated data alone

- **WHEN** a snapshot containing one project is imported in merge mode into a tenant that has two projects
- **THEN** the imported project is created or updated and the other project is unchanged

#### Scenario: Replace removes entities absent from the snapshot

- **WHEN** a snapshot is imported in replace mode
- **THEN** entities within the snapshot's scope that the snapshot does not contain are removed

#### Scenario: Mode must be stated

- **WHEN** an import is requested without specifying a mode
- **THEN** the import does not proceed destructively and the caller is told a mode is required

### Requirement: Identifier remapping

Import SHALL support remapping identifiers so that a snapshot can be loaded into a different tenant or a different database without colliding with existing records, while preserving the relationships expressed in the snapshot.

#### Scenario: Load into a different tenant

- **WHEN** a snapshot exported from one tenant is imported into another tenant
- **THEN** the entities are created under the target tenant and their subtask and dependency relationships point at the newly created entities

#### Scenario: Colliding identifiers are remapped

- **WHEN** a snapshot contains an identifier that already exists in the target database for a different record
- **THEN** the imported record receives a new identifier and every reference to it within the snapshot is updated consistently

#### Scenario: Remapping is reported

- **WHEN** an import remaps identifiers
- **THEN** the result reports the mapping from snapshot identifiers to created identifiers

### Requirement: Task identity is the task's own identifier

Import SHALL match an incoming task record against an existing task by the task's own stable identifier, never by the human-facing project-and-sequence reference, because a sequence number is a per-project counter and two databases can independently assign the same reference to unrelated tasks.

#### Scenario: A shared project-and-sequence reference does not merge unrelated tasks

- **WHEN** a snapshot is imported into a project that already has its own, unrelated task at the same sequence number
- **THEN** the imported task is created as its own task, the existing task is untouched, and the imported task is assigned a sequence number that is not already taken

#### Scenario: A record with no identifier is always created

- **WHEN** a task record carries no identifier
- **THEN** import creates it as a new task rather than attempting to match it against an existing one

#### Scenario: A matching identifier in a different project is not treated as the same task

- **WHEN** an imported task's identifier matches an existing task that belongs to a different project than the one being imported into
- **THEN** the existing task is left untouched and the imported task is created as its own task, identifier collisions being resolved the same way any other colliding identifier is

### Requirement: Soft-deleted tasks travel as tombstones

Export SHALL include soft-deleted tasks, carrying their deletion time, so that a deletion is visible to whatever imports the snapshot. Import SHALL apply a deletion it receives to a matching existing task rather than silently discarding the record.

#### Scenario: A deleted task is exported

- **WHEN** a tenant containing a soft-deleted task is exported
- **THEN** the snapshot contains a record for that task carrying its deletion time

#### Scenario: An imported deletion is applied to an existing live task

- **WHEN** a snapshot containing a deleted task is imported and the target has a live task matching that identifier, last updated no more recently than the snapshot's record
- **THEN** the target's task is deleted

#### Scenario: A deletion for a task the target never had is still recorded

- **WHEN** a snapshot containing a deleted task is imported into a target with no matching task
- **THEN** the task is created already deleted, so its identity and sequence number survive the round trip

### Requirement: Import does not silently overwrite newer data

Import SHALL compare an incoming record's last-updated time against the matching existing record's, and SHALL skip applying a record that is not newer than what already exists, rather than overwriting it. Every such skip SHALL appear in the result the caller receives.

#### Scenario: An older update is skipped, not applied

- **WHEN** a snapshot record is imported over an existing task that was updated more recently than the snapshot's record
- **THEN** the existing task is left unchanged, and the result reports the task as skipped with the reason

#### Scenario: An older deletion does not remove newer work

- **WHEN** a snapshot's deletion of a task is imported and the target's task was updated more recently than the deletion
- **THEN** the target's task remains, and the result reports the deletion as skipped

#### Scenario: A stale skip is visible in a dry run

- **WHEN** a dry run would skip a record as stale
- **THEN** the dry run's result reports the skip in the same way a real import would

### Requirement: Import audits record what actually happened

A successful import SHALL record, for each task it touches, whether the task was created, updated, deleted, or left unchanged, and SHALL NOT record a creation for a task that already existed.

#### Scenario: Updating an existing task is audited as an update

- **WHEN** an import updates a task that already exists in the target
- **THEN** the audit entry for that task records an update, not a creation

#### Scenario: Deleting a task is audited as a deletion

- **WHEN** an import applies a task's deletion
- **THEN** the audit entry for that task records a deletion, not a creation

#### Scenario: Reimporting unchanged data writes no audit entry

- **WHEN** a snapshot is imported a second time with nothing changed since the first import
- **THEN** no audit entry or event is written for the tasks that did not change

### Requirement: Round-trip fidelity

Exporting a tenant, importing that snapshot into an empty target, and exporting the target again SHALL produce content equivalent to the first snapshot, differing only in identifiers that were deliberately remapped and in timestamps recording the import itself.

#### Scenario: Round trip preserves content

- **WHEN** a tenant is exported, imported into an empty tenant, and exported again
- **THEN** the second snapshot is equivalent to the first once remapped identifiers are accounted for

#### Scenario: Custom field values survive the round trip

- **WHEN** tasks carrying typed custom field values are round-tripped
- **THEN** each value and its type are preserved

#### Scenario: Relationships survive the round trip

- **WHEN** tasks with subtasks and dependencies are round-tripped
- **THEN** the same relationship graph is present in the target

### Requirement: Validation before any write

Import SHALL validate the entire snapshot, including format version, schema shape, referential integrity, workflow state legality, and custom field typing, before writing anything. A snapshot that fails validation SHALL result in no change.

#### Scenario: Malformed document is rejected

- **WHEN** a syntactically invalid document is imported
- **THEN** the import fails with a parse error and no record is created

#### Scenario: Broken reference is rejected

- **WHEN** a snapshot references a workflow state that the snapshot's workflow does not define
- **THEN** the import fails with an error identifying the offending task and state, and nothing is written

#### Scenario: All errors are reported together

- **WHEN** a snapshot contains several validation errors
- **THEN** the import reports the errors it found rather than stopping at the first

### Requirement: Transactional import

An import SHALL be applied atomically: either every change it describes is committed or none is. A failure partway through SHALL leave no partial state.

#### Scenario: Failure leaves no partial state

- **WHEN** an import fails after some entities have been prepared
- **THEN** none of those entities is present afterwards

#### Scenario: Interruption leaves no partial state

- **WHEN** the process is terminated during an import
- **THEN** the target contains either the complete import or none of it

### Requirement: Uploads are read outside the write transaction

An import SHALL read its whole snapshot before it opens a write transaction, so a slow or stalled client cannot hold the store's write lock. A snapshot too large to hold in memory SHALL spill to temporary storage, and one larger than the configured bound SHALL be rejected with a validation error naming the bound.

#### Scenario: A stalled upload holds no write lock

- **WHEN** a client uploads a snapshot slowly
- **THEN** other writers are unaffected until the upload has been read in full

#### Scenario: An oversized snapshot is refused

- **WHEN** an uploaded snapshot exceeds the size bound
- **THEN** the import is rejected with a validation error naming the bound and nothing is written

### Requirement: Import dry run

Import SHALL support a dry run that reports exactly what would be created, updated, and skipped, without writing anything.

#### Scenario: Dry run writes nothing

- **WHEN** an import is run with the dry run option
- **THEN** a per-entity summary of creations, updates, and skips is reported and the target is unchanged

#### Scenario: Dry run surfaces validation errors

- **WHEN** a dry run is performed on an invalid snapshot
- **THEN** the same validation errors a real import would report are reported

#### Scenario: Dry run predicts the real run

- **WHEN** a dry run is followed by a real import of the same snapshot with no intervening changes
- **THEN** the real import produces the counts the dry run predicted

### Requirement: Imports emit audit entries and events

A successful import SHALL emit audit entries and outbox events for the entities it creates and updates, attributed to the system actor rather than to the operator who invoked it.

#### Scenario: Created entities are audited

- **WHEN** an import creates tasks
- **THEN** audit entries exist for those creations with the system actor

#### Scenario: Subscribers observe imported changes

- **WHEN** an import commits while a subscriber is connected
- **THEN** the subscriber receives events for the imported entities

#### Scenario: Dry run emits nothing

- **WHEN** an import dry run completes
- **THEN** no audit entry and no event is emitted

### Requirement: Tenant isolation on import and export

Export SHALL return only data belonging to the caller's authorized tenant, and import SHALL write only into the authorized target tenant. A snapshot SHALL NOT be able to direct writes into a tenant the caller is not authorized for.

#### Scenario: Export is scoped

- **WHEN** a caller authorized for one tenant exports
- **THEN** no other tenant's data appears in the snapshot

#### Scenario: Snapshot cannot redirect the target

- **WHEN** an imported snapshot names a tenant the caller is not authorized for
- **THEN** the import is refused and nothing is written outside the authorized tenant
