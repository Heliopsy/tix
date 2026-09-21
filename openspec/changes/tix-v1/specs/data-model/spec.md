## ADDED Requirements

### Requirement: Core entity set

The system SHALL persist the following entities: `tenants`, `tenant_domains`, `tenant_members`,
`actors`, `users`, `sessions`, `api_tokens`, `workflows`, `projects`, `field_defs`, `tasks`,
`task_deps`, `tags`, `task_tags`, `comments`, `artifacts`, `events`, `audit_entries`,
`webhook_endpoints`, `webhook_deliveries`, `retention_policies`, `external_refs`, `sync_sources`,
and `schema_migrations`. Every entity SHALL be reachable through the service layer and SHALL NOT
require direct database access to read or write.

#### Scenario: Fresh database contains every entity

- **WHEN** migrations are applied to an empty database
- **THEN** every entity listed above exists with its documented columns and constraints

#### Scenario: Entity reachable through the service

- **WHEN** a caller needs to read or modify any persisted entity
- **THEN** a service method exists for that operation and no direct SQL is required of the caller

#### Scenario: Referential integrity is enforced

- **WHEN** a row is created that references a parent row which does not exist
- **THEN** the write is rejected with a validation error and nothing is persisted

### Requirement: Identifier format

Every entity SHALL be identified by a string identifier that is lexicographically sortable by
creation time and generated from a cryptographically secure random source. Identifiers SHALL be
unique across the deployment and SHALL NOT be reused after deletion.

#### Scenario: Identifiers sort by creation order

- **WHEN** three entities are created in sequence
- **THEN** sorting their identifiers as strings yields the same order as sorting by creation time

#### Scenario: Identifiers are unpredictable

- **WHEN** an identifier is generated
- **THEN** its random component comes from a cryptographically secure source and knowing prior identifiers does not allow the next one to be predicted

#### Scenario: Identifier collision

- **WHEN** an insert is attempted with an identifier that already exists
- **THEN** the write fails rather than overwriting the existing row

### Requirement: Human-readable task references

Each task SHALL carry a monotonically increasing sequence number stored in a `seq` column that is
unique per project. The system SHALL expose a human reference formed from the project key and the
sequence number, such as `infra-42`, and SHALL accept that reference anywhere a task identifier is
accepted.

#### Scenario: Sequence starts at one per project

- **WHEN** the first task is created in a project with key `infra`
- **THEN** it is assigned sequence 1 and is addressable as `infra-1`

#### Scenario: Sequences are independent across projects

- **WHEN** a task is created in project `infra` and another in project `web`
- **THEN** both may hold the same sequence number and their human references remain distinct

#### Scenario: Reference used in place of an identifier

- **WHEN** a caller supplies `infra-42` where a task identifier is expected
- **THEN** the system resolves it to the corresponding task within the current tenant

#### Scenario: Sequence is not reused after deletion

- **WHEN** the task holding the highest sequence in a project is deleted and a new task is created
- **THEN** the new task receives a sequence higher than the deleted one

### Requirement: Soft delete

Tenant-owned entities that support deletion SHALL be marked deleted by setting a `deleted_at`
timestamp rather than removing the row. Ordinary reads and list operations SHALL exclude
soft-deleted rows, and the system SHALL provide an explicit way to include or restore them.

#### Scenario: Deleted row disappears from normal reads

- **WHEN** a task is deleted and the task list is requested
- **THEN** the task is absent from the results and its `deleted_at` value is set

#### Scenario: Deleted rows are retrievable on request

- **WHEN** a caller explicitly requests deleted entities
- **THEN** soft-deleted rows are returned together with their deletion timestamp

#### Scenario: Restore

- **WHEN** a soft-deleted task is restored
- **THEN** `deleted_at` is cleared and the task reappears in normal reads with its prior content intact

#### Scenario: References to deleted rows stay resolvable

- **WHEN** an audit entry or event refers to a soft-deleted task
- **THEN** the reference still resolves and history is not broken

### Requirement: Optimistic concurrency

Mutable entities SHALL carry a `version` column that increments on every successful update. An
update that supplies an expected version SHALL be rejected when the stored version differs, and the
caller SHALL be told the update conflicted rather than having its write silently applied.

#### Scenario: Version increments on update

- **WHEN** an entity is updated successfully
- **THEN** its `version` is greater than the value read before the update

#### Scenario: Stale write is rejected

- **WHEN** two callers read the same task at version 4 and both submit an update expecting version 4
- **THEN** the first update succeeds and the second is rejected with a conflict error, leaving the first writer's data intact

#### Scenario: Version omitted

- **WHEN** an update is submitted without an expected version
- **THEN** the update applies last-writer-wins and still increments the version

### Requirement: Subtasks are containment

A task SHALL be able to reference another task as its parent through a `parent_id` column,
expressing that the child is part of the parent's scope. The system SHALL reject a parent
assignment that would create a cycle, and SHALL reject a parent in a different project.

#### Scenario: Child task under a parent

- **WHEN** a task is given a parent
- **THEN** the parent lists it among its children and the child reports its parent

#### Scenario: Cycle rejected

- **WHEN** a task is assigned a parent that is already one of its own descendants
- **THEN** the assignment is rejected with a validation error

#### Scenario: Deleting a parent

- **WHEN** a task with children is deleted
- **THEN** the system reports the affected children and does not leave a child pointing at a row that normal reads cannot resolve

### Requirement: Dependencies are ordering

The system SHALL record dependencies between tasks in a `task_deps` table expressing that one task
must complete before another may start. Dependencies SHALL be distinct from parent/child
containment: a dependency SHALL NOT imply containment and containment SHALL NOT imply ordering. The
system SHALL reject a dependency that creates a cycle.

#### Scenario: Dependency blocks a task

- **WHEN** task B depends on task A and task A is not complete
- **THEN** task B is reported as blocked and is not offered as an unblocked task

#### Scenario: Dependency across projects

- **WHEN** a dependency is created between tasks in two different projects of the same tenant
- **THEN** it is accepted, because dependencies express ordering rather than containment

#### Scenario: Subtask is not automatically a dependency

- **WHEN** a task has an incomplete child
- **THEN** the child relationship alone does not mark the parent blocked unless a dependency also exists

#### Scenario: Dependency cycle rejected

- **WHEN** a dependency is added that would make a task transitively depend on itself
- **THEN** the write is rejected with a validation error

### Requirement: Timestamp storage and ordering

All timestamps SHALL be stored in UTC. On SQLite they SHALL be stored as RFC3339 UTC strings of
fixed width so that lexicographic string ordering equals chronological ordering. Timestamp
comparisons used for lease expiry and pagination cursors SHALL produce identical results on every
supported engine.

#### Scenario: Lexicographic ordering matches chronological ordering

- **WHEN** rows written at different times are sorted by their stored timestamp string on SQLite
- **THEN** the resulting order matches the order in which they were written

#### Scenario: Local time input

- **WHEN** a caller supplies a timestamp with a non-UTC offset
- **THEN** it is converted to UTC before storage and returned as UTC

#### Scenario: Cross-engine comparison

- **WHEN** the same lease-expiry comparison is evaluated on SQLite and PostgreSQL over equivalent data
- **THEN** both engines select the same rows

### Requirement: Custom field values

Tasks SHALL store custom field values in a single JSON column. Every value SHALL be validated
against the matching `field_defs` row for its tenant and project before it is written, covering
type, required status, and any enumerated options. Values for undefined fields SHALL be rejected.

#### Scenario: Value matching its definition

- **WHEN** a value of the declared type is written for a defined field
- **THEN** the write succeeds and the value is returned unchanged on read

#### Scenario: Type mismatch

- **WHEN** a string is written to a field defined as a number
- **THEN** the write is rejected with a validation error naming the field

#### Scenario: Unknown field

- **WHEN** a value is written under a key with no matching field definition
- **THEN** the write is rejected

#### Scenario: Drift from out-of-band edits

- **WHEN** the database is edited outside the service and holds a custom field value the service would reject
- **THEN** the diagnostic check reports the drift and names the affected tasks and fields

### Requirement: Migrations are forward-only

Schema changes SHALL be applied as an ordered sequence of forward-only migration files. Each file
SHALL be applied within a single transaction, and each successful application SHALL be recorded in
the `schema_migrations` table. The system SHALL NOT provide automatic down migrations.

#### Scenario: Migration applied once

- **WHEN** migrations run against a database that already recorded migration 5
- **THEN** migration 5 is skipped and only later migrations are applied

#### Scenario: Failing migration

- **WHEN** a migration file fails partway through
- **THEN** its transaction is rolled back, no row is recorded for it, and the process reports the failure without applying later migrations

#### Scenario: Database newer than the binary

- **WHEN** the database records a migration the running binary does not contain
- **THEN** the system refuses to operate and reports the version mismatch

### Requirement: External identity references

The system SHALL record an external identity for every entity imported from another system in an
`external_refs` table carrying `tenant_id`, `entity_type`, `entity_id`, `system`, `external_id`,
`external_url`, `external_version`, and `last_synced_at`. The pair of `system` and `external_id`
SHALL be unique per tenant and entity type.

#### Scenario: Re-import updates rather than duplicates

- **WHEN** an import runs a second time over a source record that already has an external reference
- **THEN** the existing entity is updated and no duplicate entity is created

#### Scenario: External link is retained

- **WHEN** an imported task is read
- **THEN** its external system, external identifier, external URL, and last sync time are available

#### Scenario: Unchanged source record

- **WHEN** a refresh finds a source record whose external version matches the stored value
- **THEN** the entity is left unmodified and reported as skipped

### Requirement: Events and audit entries are append-only

Rows in `events` and `audit_entries` SHALL be written only by mutations and SHALL NOT be updated or
individually deleted by the service. They SHALL be removed only by retention pruning. Every
mutation SHALL write its audit entry and event in the same transaction as the domain rows it
changes.

#### Scenario: Mutation writes its trail atomically

- **WHEN** a task is updated successfully
- **THEN** the task row, an audit entry, and an event are all committed together

#### Scenario: Failed mutation writes nothing

- **WHEN** a task update fails after the domain row was staged
- **THEN** no audit entry and no event exist for that attempt

#### Scenario: No update path

- **WHEN** any caller attempts to modify an existing audit entry through the service
- **THEN** no such operation is available and the entry remains unchanged

### Requirement: Tenant ownership on tenant-owned tables

Every tenant-owned table SHALL carry a `tenant_id` column, and every row in such a table SHALL
belong to exactly one tenant. Rows SHALL NOT be moved between tenants by an update.

#### Scenario: Row created with the caller's tenant

- **WHEN** an entity is created
- **THEN** its `tenant_id` is the tenant resolved for the request and is not taken from caller-supplied input

#### Scenario: Tenant reassignment refused

- **WHEN** an update attempts to change an existing row's `tenant_id`
- **THEN** the change is rejected and the row keeps its original tenant

### Requirement: Uniqueness within a tenant

Human-facing keys SHALL be unique within their tenant rather than globally. Project keys and tag
names SHALL be unique per tenant, and task sequence numbers SHALL be unique per project.

#### Scenario: Same project key in two tenants

- **WHEN** two different tenants each create a project with key `infra`
- **THEN** both creations succeed

#### Scenario: Duplicate project key in one tenant

- **WHEN** a tenant creates a second project with a key it already uses
- **THEN** the creation is rejected with a conflict error

#### Scenario: Duplicate tag name

- **WHEN** a tag name that already exists in the tenant is created again
- **THEN** the creation is rejected and the existing tag is unchanged

### Requirement: Project colour and icon

A project SHALL carry an optional colour and an optional icon so that rows belonging to different
projects can be told apart at a glance. The colour SHALL be a token drawn from a fixed palette
defined by the system rather than a free-form colour value, and a colour outside the palette SHALL
be rejected with a validation error by the service layer, on every access path. The icon SHALL be a
short printable token, at most two characters, which admits a single emoji or a brief monogram and
requires no icon font or asset pipeline. Both SHALL default to empty, SHALL be settable at creation
and afterwards, and SHALL be clearable by setting them to empty.

#### Scenario: Colour and icon round trip

- **WHEN** a project is created with a palette colour and an icon
- **THEN** reading the project back returns the same colour and icon

#### Scenario: Colour outside the palette is refused

- **WHEN** a project is created or updated with a colour that is not in the palette
- **THEN** the write is rejected with a validation error and nothing is persisted

#### Scenario: Icon longer than the limit is refused

- **WHEN** a project is created or updated with an icon longer than two characters
- **THEN** the write is rejected with a validation error

#### Scenario: Both are clearable

- **WHEN** a project that has a colour and an icon is updated with an empty colour and an empty icon
- **THEN** the project keeps its other fields and carries neither a colour nor an icon

#### Scenario: Existing projects are unaffected by the migration

- **WHEN** the migration adding the colour and icon columns is applied to a database holding projects
- **THEN** every existing project carries an empty colour and an empty icon and every other field is unchanged
