## ADDED Requirements

### Requirement: Workflow definition

A workflow SHALL be a named state machine consisting of a set of states and a set of allowed transitions between those states. A workflow SHALL belong to exactly one tenant and SHALL have a name that is unique within that tenant.

#### Scenario: Create a workflow

- **WHEN** a workflow is created with a name, a set of states, and a set of transitions
- **THEN** the workflow is stored and can be retrieved by name or identifier, reporting its states and transitions

#### Scenario: Duplicate workflow name

- **WHEN** a workflow is created with a name already used by another workflow in the same tenant
- **THEN** the request is rejected with a conflict error and no workflow is created

#### Scenario: Workflow with no states

- **WHEN** a workflow is created with an empty state set
- **THEN** the request is rejected with a validation error

#### Scenario: Transition naming an unknown state

- **WHEN** a workflow is created whose transition references a state not present in the workflow's state set
- **THEN** the request is rejected with a validation error identifying the unknown state

### Requirement: Builtin default workflow

The system SHALL ship a builtin default workflow containing the states `todo`, `doing`, `blocked`, `done`, and `cancelled`, so that a fresh installation is usable without configuring any workflow.

A workflow belongs to exactly one tenant, so the builtin workflow SHALL be seeded per tenant, at the
moment the tenant is created, rather than by a schema migration: a migration runs once per database
and cannot reach a tenant created afterwards. Seeding SHALL be idempotent, so a repeated start or a
retry after a partial failure neither duplicates the workflow nor fails.

#### Scenario: New tenant seeded

- **WHEN** a tenant is created
- **THEN** that tenant holds the builtin default workflow and a project can be created in it without naming a workflow

#### Scenario: Seeding repeated

- **WHEN** the seeding path runs again against a tenant that already holds the builtin default workflow
- **THEN** it succeeds and the tenant still holds exactly one workflow under that key

#### Scenario: Fresh installation

- **WHEN** a project is created on a fresh installation with no workflow configured
- **THEN** the project is assigned the builtin default workflow and tasks can be created and transitioned without further configuration

#### Scenario: Default terminal states

- **WHEN** the builtin default workflow is inspected
- **THEN** `done` and `cancelled` are reported as terminal states and `todo` is reported as the initial state

### Requirement: Project workflow assignment

Each project SHALL be assigned exactly one workflow. Every task in a project SHALL be governed by that project's workflow.

#### Scenario: Assign a workflow at project creation

- **WHEN** a project is created naming an existing workflow
- **THEN** the project is assigned that workflow and task status values are validated against it

#### Scenario: Assign an unknown workflow

- **WHEN** a project is created or updated naming a workflow that does not exist in the tenant
- **THEN** the request is rejected with a not-found error and the project's workflow is unchanged

#### Scenario: Reassign a project workflow

- **WHEN** a project's workflow is changed to another workflow whose state set contains every status currently in use by that project's tasks
- **THEN** the reassignment succeeds and subsequent transitions are validated against the new workflow

### Requirement: State flags

Each state in a workflow SHALL carry flags describing its behaviour, including whether the state is terminal and whether a task in that state SHALL be reverted when its lease expires. A workflow SHALL declare exactly one initial state.

#### Scenario: Terminal state reported

- **WHEN** a task is in a state flagged terminal
- **THEN** the task is reported as complete for the purpose of dependency resolution and is not offered by queue operations

#### Scenario: Revert-on-lease-expiry marker

- **WHEN** a lease expires on a task in a state flagged revert-on-lease-expiry
- **THEN** the task's status is reverted to the revert target declared by the workflow and the change is recorded

#### Scenario: State without revert marker

- **WHEN** a lease expires on a task in a state not flagged revert-on-lease-expiry
- **THEN** the task's status is left unchanged and only the claim is released

### Requirement: Transition validation

A status change SHALL be permitted only when a transition from the task's current state to the requested state exists in the project's workflow. A transition that is not present in the workflow SHALL be rejected.

#### Scenario: Allowed transition

- **WHEN** a task in `todo` is transitioned to `doing` and the workflow allows that transition
- **THEN** the transition succeeds and the task's status becomes `doing`

#### Scenario: Disallowed transition

- **WHEN** a task in `todo` is transitioned to `done` and no such transition exists in the workflow
- **THEN** the request is rejected with a validation error naming the current state, the requested state, and the allowed targets, and the task's status is unchanged

#### Scenario: Transition to an unknown state

- **WHEN** a task is transitioned to a status that is not a state in the project's workflow
- **THEN** the request is rejected with a validation error and the task's status is unchanged

#### Scenario: Listing available transitions

- **WHEN** the available transitions for a task are requested
- **THEN** only the transitions whose source is the task's current state are returned

### Requirement: Transition requirements

A transition MAY declare that it requires a specific authorization scope and MAY declare that it requires a comment. A transition request that does not satisfy a declared requirement SHALL be rejected.

#### Scenario: Scope-restricted transition denied

- **WHEN** an actor without the scope required by a transition attempts that transition
- **THEN** the request is rejected with a permission error and the task's status is unchanged

#### Scenario: Scope-restricted transition allowed

- **WHEN** an actor holding the scope required by a transition attempts that transition
- **THEN** the transition succeeds

#### Scenario: Comment required and omitted

- **WHEN** a transition that requires a comment is attempted without a comment
- **THEN** the request is rejected with a validation error stating that a comment is required

#### Scenario: Comment required and supplied

- **WHEN** a transition that requires a comment is attempted with a comment
- **THEN** the transition succeeds and the comment is recorded against the task in the same operation

### Requirement: Custom field definitions

A custom field definition SHALL be scoped to a project and SHALL declare a key, a display name, and a type drawn from `string`, `text`, `int`, `float`, `bool`, `date`, `datetime`, `enum`, `actor`, and `json`. A field key SHALL be unique within its project.

#### Scenario: Define a typed field

- **WHEN** a field definition is created for a project with key `severity` and type `enum`
- **THEN** the definition is stored and reported when the project's field definitions are listed

#### Scenario: Unknown type

- **WHEN** a field definition is created with a type outside the supported set
- **THEN** the request is rejected with a validation error

#### Scenario: Duplicate key within a project

- **WHEN** a field definition is created with a key already defined in the same project
- **THEN** the request is rejected with a conflict error

#### Scenario: Same key in a different project

- **WHEN** a field definition with key `severity` is created in a project where that key is unused, while another project already defines `severity`
- **THEN** the definition is created and the two definitions remain independent

### Requirement: Enum field options and defaults

An `enum` field definition SHALL declare a non-empty list of permitted options. Any field definition MAY declare a default value, which SHALL be applied when a task is created without an explicit value for that field.

#### Scenario: Enum without options

- **WHEN** an `enum` field definition is created with no options
- **THEN** the request is rejected with a validation error

#### Scenario: Default applied on create

- **WHEN** a task is created in a project whose field definition declares a default and no value is supplied for that field
- **THEN** the task is created carrying the declared default value

#### Scenario: Default incompatible with the declared type

- **WHEN** a field definition declares a default value that does not validate against its own type or option list
- **THEN** the definition is rejected with a validation error

### Requirement: Custom field value validation

A custom field value SHALL be validated against its definition whenever it is written. A value that does not conform to the definition's type, option list, or other declared constraints SHALL be rejected and SHALL NOT be stored.

#### Scenario: Value of the wrong type

- **WHEN** a task is created or updated with a non-numeric value for a field defined as `int`
- **THEN** the request is rejected with a validation error naming the field and no change is written

#### Scenario: Value outside the enum options

- **WHEN** a task is updated with a value not present in an `enum` field's option list
- **THEN** the request is rejected with a validation error listing the permitted options

#### Scenario: Value for an undefined field

- **WHEN** a task is written with a custom field key that has no definition in the task's project
- **THEN** the request is rejected with a validation error naming the unknown key

#### Scenario: Conforming value

- **WHEN** a task is written with a value conforming to every referenced field definition
- **THEN** the values are stored and returned unchanged on subsequent reads

### Requirement: Required custom fields

A field definition MAY be marked required. A required field SHALL be enforced when a task is created, and SHALL be enforced on a transition where the field definition or the transition declares that requirement.

#### Scenario: Required field missing at create

- **WHEN** a task is created in a project with a required field for which no value is supplied and no default exists
- **THEN** the request is rejected with a validation error naming the missing field and no task is created

#### Scenario: Required field enforced at transition

- **WHEN** a transition declares a field requirement and the task has no value for that field
- **THEN** the transition is rejected with a validation error and the task's status is unchanged

#### Scenario: Required field satisfied

- **WHEN** a task carries values for every field required at the point of the operation
- **THEN** the operation proceeds normally

#### Scenario: Marking an existing field required

- **WHEN** an existing field definition is marked required while tasks without a value for that field exist
- **THEN** the change is accepted, existing tasks are not modified, and the requirement applies to subsequent creates and declared transitions

### Requirement: Indexed custom fields

A field definition MAY be marked indexed. A field marked indexed SHALL be efficiently filterable. A field not marked indexed SHALL remain filterable, but without an efficiency guarantee, and filtering it SHALL NOT be reported as an error.

#### Scenario: Filtering an indexed field

- **WHEN** tasks are filtered by an indexed custom field value
- **THEN** matching tasks are returned and the query does not degrade with the number of tasks in the project

#### Scenario: Filtering a non-indexed field

- **WHEN** tasks are filtered by a non-indexed custom field value
- **THEN** matching tasks are returned correctly and no error is raised

#### Scenario: Marking a field indexed later

- **WHEN** an existing field definition is marked indexed
- **THEN** existing values for that field become filterable under the efficiency guarantee without any task being modified

### Requirement: Workflow editing safety

Editing a workflow in a way that would leave existing tasks in a state no longer present in the workflow SHALL be rejected, unless the request supplies an explicit migration mapping each removed state to a surviving state.

#### Scenario: Removing a state still in use

- **WHEN** a state is removed from a workflow while tasks governed by that workflow are in that state
- **THEN** the request is rejected with a conflict error reporting the affected state and the number of tasks in it

#### Scenario: Removing a state with an explicit migration

- **WHEN** a state is removed and the request supplies a migration mapping that state to a surviving state
- **THEN** the workflow is updated and every affected task is moved to the mapped state in the same operation

#### Scenario: Removing an unused state

- **WHEN** a state with no tasks in it is removed from a workflow
- **THEN** the update succeeds without a migration

#### Scenario: Removing a transition

- **WHEN** a transition is removed from a workflow
- **THEN** the update succeeds, existing task statuses are unchanged, and subsequent attempts to make that transition are rejected

### Requirement: Workflow export and import

A workflow definition, including its states, transitions, and the field definitions associated with it, SHALL be exportable to a portable document and importable from one.

#### Scenario: Export a workflow

- **WHEN** a workflow is exported
- **THEN** a document is produced containing its states, state flags, transitions, transition requirements, and field definitions

#### Scenario: Round trip

- **WHEN** an exported workflow document is imported under a new name
- **THEN** the resulting workflow has states, flags, transitions, and field definitions equivalent to the original

#### Scenario: Import a malformed document

- **WHEN** a workflow document that fails validation is imported
- **THEN** the import is rejected with a validation error and no workflow or field definition is created

#### Scenario: Import over an existing name

- **WHEN** a workflow document is imported using the name of an existing workflow without an instruction to replace it
- **THEN** the import is rejected with a conflict error and the existing workflow is unchanged

### Requirement: Workflow deletion safety

A workflow SHALL NOT be deleted while any project is assigned to it.

#### Scenario: Delete a workflow in use

- **WHEN** deletion is requested for a workflow assigned to at least one project
- **THEN** the request is rejected with a conflict error naming the projects still using it

#### Scenario: Delete an unused workflow

- **WHEN** deletion is requested for a workflow assigned to no project
- **THEN** the workflow is deleted and is no longer listed

#### Scenario: Delete the builtin default workflow

- **WHEN** deletion is requested for the builtin default workflow
- **THEN** the request is rejected and the builtin workflow remains available
