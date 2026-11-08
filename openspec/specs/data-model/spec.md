# data-model Specification

## Purpose

Names the persisted entities and the relationships between them. It is the shape every other capability is written against.

## Requirements

### Requirement: Core entity set

The system SHALL persist the following entities: `tenants`, `tenant_domains`, `tenant_members`,
`actors`, `users`, `sessions`, `api_tokens`, `workflows`, `projects`, `field_defs`, `tasks`,
`task_deps`, `tags`, `task_tags`, `comments`, `artifacts`, `events`, `audit_entries`,
`webhook_endpoints`, `webhook_deliveries`, `retention_policies`, `external_refs`, `sync_sources`,
and `schema_migrations`.

#### Scenario: Fresh database contains every entity

- **WHEN** migrations are applied to an empty database
- **THEN** every entity listed above exists with its documented columns and constraints

#### Scenario: Referential integrity is enforced

- **WHEN** a row is created that references a parent row which does not exist
- **THEN** the write is rejected with a validation error and nothing is persisted

### Requirement: Every entity is reachable through the service layer

Every entity SHALL be reachable through the service layer and SHALL NOT require direct database
access to read or write.

#### Scenario: Entity reachable through the service

- **WHEN** a caller needs to read or modify any persisted entity
- **THEN** a service method exists for that operation and no direct SQL is required of the caller

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
- **THEN** migration 5 is skipped and every migration the database has not recorded is applied

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
projects can be told apart at a glance. Both the colour and the icon SHALL default to empty, SHALL
be settable at creation and afterwards, and SHALL be clearable by setting them to empty.

#### Scenario: Colour and icon round trip

- **WHEN** a project is created with a palette colour and an icon
- **THEN** reading the project back returns the same colour and icon

#### Scenario: Both are clearable

- **WHEN** a project that has a colour and an icon is updated with an empty colour and an empty icon
- **THEN** the project keeps its other fields and carries neither a colour nor an icon of any kind

#### Scenario: Existing projects are unaffected by the migration

- **WHEN** the migration adding the colour and icon columns, or the one adding built-in and uploaded icons, is applied to a database holding projects
- **THEN** every existing project keeps the colour and text icon it had, carries no built-in icon and no image, and every other field is unchanged

#### Scenario: The project listing shows both

- **WHEN** projects are listed in the human-readable table format
- **THEN** the table carries a colour column and an icon column, the colour is drawn as a sample in
  that colour when colour is enabled and as the bare colour name or hexadecimal value when it is not, a built-in
  icon is shown as its fallback glyph and its name, an uploaded image as a fixed mark, and a project
  with neither renders both columns empty

#### Scenario: Appearance survives a snapshot round trip

- **WHEN** a project carrying a colour and a text or built-in icon is exported and the snapshot is imported elsewhere
- **THEN** the imported project carries the same colour and the same icon

#### Scenario: Appearance travels with a project template

- **WHEN** a project carrying a colour and a text or built-in icon is exported as a bundle component and the bundle is applied
- **THEN** the resulting project carries the same colour and the same icon, and a bundle naming a
  colour that is neither in the palette nor a custom colour, an over-long icon or an unknown built-in icon is rejected before
  anything is written

### Requirement: A project colour is drawn from a fixed palette

The colour SHALL be either a token drawn from a fixed palette defined by the system, or a custom
colour written as `#` followed by six hexadecimal digits. The system SHALL store a palette token or a
custom colour in lowercase, and SHALL reject any other value, including short hexadecimal forms,
colour names outside the palette and functional notations, with a validation error raised by the
service layer on every access path.

#### Scenario: A custom colour round trips in lowercase

- **WHEN** a project is created or updated with the colour `#1A7F5A`
- **THEN** reading the project back returns the colour `#1a7f5a`

#### Scenario: Colour outside the palette is refused

- **WHEN** a project is created or updated with a colour that is neither in the palette nor `#` followed by six hexadecimal digits
- **THEN** the write is rejected with a validation error and nothing is persisted

### Requirement: A project icon is a short printable token

The icon SHALL be exactly one of three kinds, or none: a text icon, which is a short printable
token of at most two characters admitting a single emoji or a brief monogram; a built-in icon,
named from a fixed set of symbols the system ships; or an uploaded image. Setting one kind SHALL
replace whichever kind the project carried, and a write naming two kinds at once SHALL be rejected.

#### Scenario: Icon longer than the limit is refused

- **WHEN** a project is created or updated with a text icon longer than two characters
- **THEN** the write is rejected with a validation error

#### Scenario: Unknown built-in icon is refused

- **WHEN** a project is created or updated with a built-in icon name the system does not ship
- **THEN** the write is rejected with a validation error naming the available icons

#### Scenario: One kind replaces another

- **WHEN** a project carrying a text icon is updated with a built-in icon
- **THEN** the project carries the built-in icon and no text icon

#### Scenario: Two kinds at once are refused

- **WHEN** a project is created with both a text icon and a built-in icon
- **THEN** the write is rejected with a validation error and nothing is persisted

### Requirement: Stars are tenant-owned rows keyed by actor and task

A star SHALL be stored as a row of `task_stars` carrying `tenant_id`, `actor_id`, `task_id` and
`created_at`, unique on the tenant, actor and task. The row SHALL be removed with its task and
with its actor, and SHALL be subject to the same tenant scoping, and on PostgreSQL the same
row-level isolation policy, as every other tenant-owned table.

#### Scenario: One row per actor and task

- **WHEN** one actor stars the same task twice
- **THEN** exactly one star row exists for that actor and task

#### Scenario: Stars are invisible across tenants

- **GIVEN** a star in one tenant
- **WHEN** a listing runs in another tenant
- **THEN** the star is neither read nor able to affect the order

### Requirement: Recurrence columns on tasks

The tasks table SHALL carry `recurrence`, a non-null text column defaulting to the empty string, and
`recurrence_next_id`, a nullable text column naming the successor. The successor column SHALL NOT be a
foreign key, so that deleting the successor never clears the guard against a second spawn. Both SHALL be
added by one forward-only migration applied identically on SQLite and PostgreSQL.

#### Scenario: Upgrade

- **WHEN** a database at the previous schema version is migrated
- **THEN** every existing task carries an empty rule and no successor

### Requirement: Uploaded project icon images

An uploaded project icon SHALL be a PNG, JPEG, WebP or SVG image of at most 64 KiB, recognised from
its bytes rather than from a type the caller declares. An SVG SHALL be refused when it carries a
script element, a `foreignObject` element, an event handler attribute, a `javascript:` URL, or a
reference to anything other than a fragment of itself or an embedded `data:` image.

#### Scenario: A declared type is not trusted

- **WHEN** a file of plain text is uploaded declaring itself `image/png`
- **THEN** the write is rejected with a validation error

#### Scenario: Oversized images are refused

- **WHEN** an image larger than 64 KiB is uploaded
- **THEN** the write is rejected with a validation error and the project is unchanged

#### Scenario: Scripted SVG is refused

- **WHEN** an SVG carrying a script element or an `onload` attribute is uploaded
- **THEN** the write is rejected with a validation error

### Requirement: Project icon image bytes are stored with their project in the tenant

The image bytes SHALL be stored in the database on both engines, in a tenant-owned table subject to
the same tenant scoping and, on PostgreSQL, the same row-level isolation policy as every other
tenant-owned table, and SHALL be removed with their project. An image SHALL be written in the same
transaction as the project change, its audit entry and its event.

#### Scenario: Another tenant cannot read the image

- **GIVEN** a project with an uploaded icon in one tenant
- **WHEN** an actor of another tenant asks for that project's icon
- **THEN** the request fails as not found and no bytes are returned

#### Scenario: Replacing the image removes the old bytes

- **WHEN** a project with an uploaded icon is given a text icon
- **THEN** reading its icon image fails as not found

#### Scenario: Imports do not carry images

- **WHEN** a snapshot holding a project with an uploaded icon is imported into a store where that project already carries an image
- **THEN** the project keeps the image it had, and an import into a store without the project creates it with no icon

### Requirement: A project carries its icon image's digest and guards reading the image

The project SHALL carry the image's content digest, and reading the image SHALL require the same
permission as reading the project.

#### Scenario: An image round trips

- **WHEN** a project is updated with a 1 KiB PNG and its icon is read back
- **THEN** the read returns the same bytes with content type `image/png`, and the project carries the image's digest and no text or built-in icon

### Requirement: Group preferences are per actor and per view

The store SHALL keep group preferences keyed by tenant, actor, view and group, holding hidden,
collapsed, starred and a position. A preference that says nothing SHALL not be stored. A project's
view and a project group SHALL be stored by the project's identifier, so a project whose key changes
keeps its arrangement. Removing an actor or a tenant SHALL remove its preferences. Preferences SHALL
not travel in snapshots.

#### Scenario: A preference reset to plain leaves no row

- **GIVEN** a reader who hid a column
- **WHEN** they show it again
- **THEN** no preference row remains for that column

#### Scenario: Readers are apart

- **WHEN** two actors of one tenant each star a different lane
- **THEN** each actor's listing of the lanes view holds only their own star

### Requirement: A project's key can be changed and its old keys stay aliases

A project update SHALL accept a new key. The new key SHALL satisfy the same rules a key at creation
satisfies, SHALL be stored in lower case, and SHALL NOT be the current key or a kept alias of another
project in the tenant; such a key SHALL be refused with a conflict error. The rename SHALL write the
project row, record the previous key as an alias of the project, and write its audit entry and a
`project.updated` event naming the previous key, all in one transaction.

#### Scenario: Renaming a key renames every reference

- **GIVEN** a project `infra` holding task `infra-8`
- **WHEN** the project's key is changed to `ops`
- **THEN** reading the task returns reference `ops-8`
- **AND** the audit log and the event stream each record one project update naming `infra` as the previous key

#### Scenario: A key in use is refused

- **GIVEN** projects `infra` and `web`
- **WHEN** `web` is renamed to `infra`
- **THEN** the rename is refused with a conflict error and nothing changes

#### Scenario: An alias of another project is refused

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** a new project is created with key `infra`, or another project is renamed to `infra`
- **THEN** the write is refused with a conflict error

#### Scenario: An invalid key is refused

- **WHEN** a project is renamed to `9lives` or to `a b`
- **THEN** the rename is refused with a validation error

### Requirement: A renamed project's old keys resolve as aliases until it is deleted

Old keys SHALL be kept as aliases for as long as the project exists. A lookup of a project or of a
task reference by an alias SHALL resolve to the renamed project, and the returned record SHALL carry
the current key and the current reference. A project MAY take back one of its own aliases, which then
stops being an alias. Deleting a project SHALL remove its aliases.

#### Scenario: Old references still resolve

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** a caller reads project `infra` or task `infra-8`
- **THEN** the project `ops` and the task `ops-8` are returned

#### Scenario: A project takes back its own old key

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** `ops` is renamed back to `infra`
- **THEN** the rename succeeds and `ops` becomes an alias of the project

#### Scenario: Aliases are isolated per tenant

- **GIVEN** tenant A renamed its project `infra` to `ops`
- **WHEN** tenant B creates a project `infra`
- **THEN** the creation succeeds and tenant A's alias does not resolve in tenant B

#### Scenario: A snapshot naming an old key updates the renamed project

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** a snapshot whose project is keyed `infra` is imported
- **THEN** the project keyed `ops` is updated and keeps the key `ops`

### Requirement: Copied-from link

A task SHALL carry an optional `copied_from_id` naming the task it was copied from. The column
SHALL NOT be a foreign key: a link to a purged task SHALL resolve to nothing rather than block
the purge or clear the link. Snapshots SHALL carry the field.

#### Scenario: Link survives the source

- **WHEN** a linked copy's source is hard deleted
- **THEN** the copy still exists and its `copied_from_id` is unchanged

#### Scenario: Unlinked by default

- **WHEN** a task is created rather than copied
- **THEN** its `copied_from_id` is empty

### Requirement: Saved filters are owned by an actor

The store SHALL keep saved filters keyed by tenant and identifier, each with its owning actor, a name, a
filter expression, a view, a sort, a shared flag and the owner's position for it. A listing for an actor
SHALL return the filters that actor owns and every shared filter of the tenant, and nothing from another
tenant. Removing an actor or a tenant SHALL remove its filters. Saved filters SHALL not travel in
snapshots.

#### Scenario: A private filter stays private

- **GIVEN** alice saved a filter without sharing it
- **WHEN** bob in the same tenant lists saved filters
- **THEN** alice's filter is not among them

#### Scenario: A shared filter is listed for the tenant

- **GIVEN** alice saved a shared filter
- **WHEN** bob in the same tenant lists saved filters
- **THEN** alice's filter is among them, after bob's own

### Requirement: Calendar feeds are stored by hash

The store SHALL keep calendar feeds in `calendar_feeds`, keyed by identifier and scoped to a tenant,
holding the owning actor, the SHA-256 hash of the secret, the filter, the creation instant and the
revocation instant. At most one unrevoked feed SHALL exist per tenant and actor. Removing an actor
or a tenant SHALL remove its feeds.

#### Scenario: One live feed per actor

- **WHEN** two unrevoked feeds are written for one actor
- **THEN** the second write fails

### Requirement: Inbox items are kept per recipient

The store SHALL keep inbox items keyed by tenant and identifier, each holding its recipient, its
kind (`mention`, `assigned`, `comment` or `status`), the task, the actor who caused it, the comment
when there is one, the statuses of a status change, a short excerpt, when it was created and when it
was read. Removing a tenant, a recipient or a task SHALL remove its items. Muted kinds SHALL be kept
per tenant and actor. Neither SHALL travel in snapshots.

#### Scenario: Purging a task removes its items

- **GIVEN** an inbox item about a task
- **WHEN** the task is purged
- **THEN** no inbox item names it

#### Scenario: Recipients are apart

- **WHEN** two actors each receive an item
- **THEN** each actor's inbox lists only their own

### Requirement: A task records its snooze

The tasks table SHALL carry `snoozed_until`, a nullable instant, and `snoozed_by`, a nullable
reference to the actor who set it that is cleared when that actor is deleted, added by migration
0023 with an index on the tenant and wake time. Existing rows SHALL upgrade with both columns
empty, meaning not snoozed.

#### Scenario: Upgrade leaves every task awake

- **GIVEN** a database at migration 0022 holding tasks
- **WHEN** it migrates
- **THEN** every task lists by default as before

### Requirement: Task templates table

Migration 0025 SHALL create `task_templates` with tenant, identifier, name, nullable project,
JSON body, due offset in seconds, use counter, creator and timestamps, on both engines, with the
tenant-scoped row-level security every scoped table has on PostgreSQL. Deleting a project SHALL
delete its templates.

#### Scenario: Upgrade

- **WHEN** a database at migration 0020 is opened by this build
- **THEN** `task_templates` exists and no task row changed

### Requirement: Invites and the password change flag are stored

`users` SHALL carry `require_password_change`, false by default. Invites SHALL live in
`user_invites`, scoped to a tenant, holding the user, email, role, inviter, token hash, creation,
expiry, acceptance and revocation instants, with at most one unaccepted, unrevoked invite per user.
Removing the user's actor or the tenant SHALL remove its invites.

#### Scenario: One pending invite per user

- **WHEN** two unaccepted, unrevoked invites are written for one user
- **THEN** the second write fails

### Requirement: The channel is stored in columns and JSON

Migration 0033 SHALL add via_kind defaulting to unknown, via_token and via_detail to audit_entries, and via_kind and via_detail to events, with an index on audit_entries over tenant, via_kind and seq, on both engines.

#### Scenario: Upgrading a database

- **WHEN** a database holding audit entries migrates to 0033
- **THEN** every existing entry reads as unknown and new entries carry their channel

### Requirement: Migrations are named by their creation time

A migration added after 0034 SHALL be named `YYYYMMDDHHMMSS_name.sql`, its version being the UTC
time it was created. The numbered migrations 0001 to 0034 SHALL keep their versions and sort before
every timestamped one. A numbered migration after 0034 SHALL be refused.

#### Scenario: Existing database upgrades without rewriting

- **GIVEN** a database that recorded every migration up to 0034
- **WHEN** a binary with timestamped migrations opens it
- **THEN** only the timestamped migrations are applied and the recorded versions are kept as they were

#### Scenario: A numbered migration after the last one

- **WHEN** the migration set holds `0035_name.sql`
- **THEN** loading the migrations fails and names the file

### Requirement: Every unrecorded migration is applied

The runner SHALL apply every migration whose version the database has not recorded, in version
order, rather than only versions above the highest recorded one.

#### Scenario: Parallel branches merge out of timestamp order

- **GIVEN** a database that applied a migration from one branch
- **WHEN** a second branch with an earlier timestamp merges and the database migrates
- **THEN** the second branch's migration is applied

### Requirement: Actors reference their account

Every actor of kind user SHALL reference its account, unique per tenant. The upgrade SHALL set that reference on every existing user actor to the account with the same identifier, and SHALL change no existing identifier.

#### Scenario: Upgrade of an existing installation

- **WHEN** a database with users created before this change is migrated
- **THEN** each user actor references the account sharing its identifier, and every task, comment, token and audit entry still names the same actor

#### Scenario: Second actor for one account in one tenant

- **WHEN** a second actor referencing the same account is written in a tenant that already holds one
- **THEN** the store refuses it as a conflict

### Requirement: Sessions record how they were minted

Each session SHALL record its sign-in method (`password`, `ssh-key` or `switch`, as the sign-in's audit entry names it) and whether a second factor was passed. Sessions existing before the upgrade SHALL read as `password` without a second factor.

#### Scenario: Old session after upgrade

- **WHEN** a session minted before the upgrade is used to switch to a tenant requiring a second factor
- **THEN** a code is asked for

### Requirement: Tenant domains carry a shared flag

Each tenant domain SHALL carry a shared flag. Domains existing before the upgrade SHALL read as not shared.

#### Scenario: Existing domain after upgrade

- **WHEN** a database with a domain bound to HO is migrated
- **THEN** the domain is not shared, and its host serves HO alone as before
