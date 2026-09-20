## ADDED Requirements

### Requirement: Audit entry written in the mutation transaction

Every mutation that changes persistent state SHALL write exactly one audit entry in the same transaction as the domain rows it describes. If the transaction rolls back, the audit entry SHALL NOT be visible, and if the transaction commits, the audit entry SHALL be visible.

#### Scenario: Successful mutation records an entry

- **WHEN** a task is updated and the operation succeeds
- **THEN** exactly one audit entry describing that update is readable, and it carries the same timestamp basis as the committed change

#### Scenario: Failed mutation records nothing

- **WHEN** a task update fails validation and the operation is rolled back
- **THEN** no audit entry for that attempted update exists

#### Scenario: Entry survives process death after commit

- **WHEN** the process terminates immediately after a mutation commits
- **THEN** the audit entry for that mutation is present when the store is read again

### Requirement: Audit entries are append-only

The system SHALL NOT expose any operation through the CLI, HTTP API, web UI, or TUI that updates or deletes an audit entry. Audit entries SHALL be readable and creatable only as a side effect of the mutation they describe.

#### Scenario: No update path exists

- **WHEN** a caller searches the API surface for an operation that modifies an existing audit entry
- **THEN** no such operation is offered on any access path

#### Scenario: Delete is rejected

- **WHEN** a caller attempts to delete a specific audit entry through any access path
- **THEN** the request is rejected and the entry remains readable

#### Scenario: Retention pruning is not an edit

- **WHEN** retention removes audit entries older than the configured window
- **THEN** remaining entries are unchanged and no partial entry is produced

### Requirement: Entry captures actor, action, and subject

Each audit entry SHALL record the acting identity, the action performed, the subject type, and the subject identifier, so that an entry identifies who did what to which record without consulting other tables.

#### Scenario: Actor is recorded

- **WHEN** a user with a known identity transitions a task
- **THEN** the audit entry names that identity as the actor

#### Scenario: Subject is addressable

- **WHEN** an audit entry is read
- **THEN** it names the subject type and the subject identifier, and that identifier resolves to the record that was changed

### Requirement: Entry captures before and after state

Each audit entry SHALL capture the state of the subject before the change and after the change, sufficient to show what differed. A create action SHALL have no before state, and a delete action SHALL have no after state.

#### Scenario: Update carries both states

- **WHEN** a task's priority is changed
- **THEN** the audit entry shows the previous priority in the before state and the new priority in the after state

#### Scenario: Create has no before state

- **WHEN** a task is created
- **THEN** the audit entry's before state is absent

#### Scenario: Delete has no after state

- **WHEN** a task is deleted
- **THEN** the audit entry's after state is absent and the before state describes the record as it was

### Requirement: Secrets are excluded from captured state

Captured before and after state SHALL NOT contain password hashes, API token values or token hashes, webhook signing secrets, session identifiers, or external system credentials. Where such a field changes, the entry SHALL record that the field changed without recording its value.

#### Scenario: Password change is recorded without the hash

- **WHEN** a user's password is changed
- **THEN** the audit entry records that the password changed and contains neither the old nor the new hash

#### Scenario: Webhook secret rotation is recorded without the secret

- **WHEN** a webhook endpoint's signing secret is rotated
- **THEN** the audit entry records the rotation and contains no secret material

### Requirement: Source attribution

Each audit entry SHALL record the access path that produced it as one of cli, api, web, tui, or system, so that entries can be distinguished by origin.

#### Scenario: Web action is attributed to web

- **WHEN** a task is edited through the web UI
- **THEN** the audit entry's source is web

#### Scenario: Import is attributed to system

- **WHEN** an external import creates tasks
- **THEN** the audit entries for those creations have source system

#### Scenario: Source distinguishes identical actions

- **WHEN** the same task is transitioned once from the CLI and once from the API by the same actor
- **THEN** the two audit entries differ in their recorded source

### Requirement: Tenant attribution and isolation

Each audit entry SHALL record the tenant it belongs to, and audit queries SHALL return only entries belonging to the caller's tenant.

#### Scenario: Entry carries its tenant

- **WHEN** an audit entry is read
- **THEN** it names the tenant that owned the changed record

#### Scenario: Other tenants are invisible

- **WHEN** a caller authorized for one tenant queries the audit log
- **THEN** no entry belonging to another tenant appears in the result, regardless of filters supplied

### Requirement: Timestamp recorded in UTC

Each audit entry SHALL record the time of the change in UTC, and entries SHALL be orderable by that time together with a stable tie-breaker so that ordering is deterministic.

#### Scenario: Ordering is stable

- **WHEN** two audit entries share the same recorded timestamp
- **THEN** repeated queries return them in the same relative order

#### Scenario: Time is returned in UTC

- **WHEN** an audit entry is returned in JSON output
- **THEN** its timestamp is expressed in UTC

### Requirement: Query by subject, actor, and time range

The system SHALL allow audit entries to be queried by subject type and identifier, by actor, by source, and by an inclusive time range, and SHALL allow these filters to be combined.

#### Scenario: Query by subject

- **WHEN** a caller queries audit entries for a specific task identifier
- **THEN** only entries whose subject is that task are returned

#### Scenario: Query by actor and time range

- **WHEN** a caller queries entries for one actor within a given time range
- **THEN** only that actor's entries whose timestamps fall inside the range are returned

#### Scenario: Empty result is not an error

- **WHEN** a query matches no entries
- **THEN** an empty result is returned with a success status

### Requirement: Keyset pagination for audit queries

Audit queries SHALL be paginated by an opaque cursor derived from the sort key and identifier, and SHALL NOT use offset-based paging. A cursor SHALL resume a listing without skipping or repeating entries.

#### Scenario: Cursor resumes cleanly

- **WHEN** a caller reads a page of audit entries and then requests the next page with the returned cursor
- **THEN** the second page continues immediately after the first with no gap and no duplicate

#### Scenario: New entries do not shift a listing

- **WHEN** new audit entries are written while a caller is paging backwards through history
- **THEN** already-returned entries are not repeated on subsequent pages

### Requirement: Per-task history view

The system SHALL render the audit entries for a single task as a chronological history showing, for each change, the actor, the time, the source, and the fields that changed with their previous and new values.

#### Scenario: History shows field changes

- **WHEN** a task has been edited three times
- **THEN** its history lists three changes in chronological order, each naming the fields that differed

#### Scenario: History is available on every access path

- **WHEN** a caller requests a task's history from the CLI, the API, the web UI, or the TUI
- **THEN** the same set of changes is presented on each

### Requirement: Attribution in no-auth mode

When the system runs with authentication disabled, an action performed with no authenticated user SHALL still be attributed to the synthesized local actor rather than being left unattributed.

#### Scenario: Local action is attributed

- **WHEN** a task is created from the CLI with authentication disabled
- **THEN** the audit entry names the synthesized local actor as the actor

#### Scenario: Attribution is never empty

- **WHEN** any audit entry is read in any mode
- **THEN** its actor field is populated

### Requirement: Audit read requires its own scope

Reading the audit log SHALL require a dedicated audit read scope that is distinct from the scopes granting read access to the underlying records. A caller able to read a task SHALL NOT thereby be able to read that task's audit entries.

#### Scenario: Missing scope is refused

- **WHEN** a caller holding task read scope but not audit read scope requests audit entries
- **THEN** the request is refused as unauthorized and no entries are returned

#### Scenario: Scope grants access

- **WHEN** a caller holding audit read scope requests audit entries for its own tenant
- **THEN** the matching entries are returned

#### Scenario: Scope does not cross tenants

- **WHEN** a caller holding audit read scope for one tenant requests entries for another tenant
- **THEN** the request is refused and no entries are returned
