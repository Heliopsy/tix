# workflows Specification

## Purpose

Defines the named state machines tasks move through, and the transitions each one allows.

## Requirements

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

### Requirement: A state category comes from a fixed vocabulary

A workflow state SHALL carry its category from a closed vocabulary of exactly six words: `todo`,
`in_progress`, `blocked`, `waiting`, `done` and `cancelled`. A state MAY carry no category at all.

The vocabulary SHALL be fixed rather than extensible, and a state SHALL NOT carry a colour of its own,
so that a category means the same thing in every deployment and on every surface.

#### Scenario: Each of the six is accepted

- **WHEN** a workflow declares a state whose category is any of the six
- **THEN** the workflow validates

#### Scenario: A state may decline to name a category

- **WHEN** a workflow declares a state carrying no category
- **THEN** the workflow validates
- **AND** the state's category is derived from what else it carries: a terminal state reads as `done`,
  the initial state as `todo`, and any other state as `in_progress`

#### Scenario: A word outside the vocabulary is refused

- **WHEN** a workflow declares a state whose category is a word the vocabulary does not contain
- **THEN** the workflow is refused as invalid
- **AND** the refusal names the state and the word it refused
- **AND** the refusal lists all six accepted categories

#### Scenario: The vocabulary cannot be edited by a caller

- **WHEN** a caller obtains the vocabulary and modifies the value it was handed
- **THEN** the next caller still receives the whole unmodified vocabulary

### Requirement: Cancelled work is categorised apart from completed work

A state for work that will not be done SHALL carry the category `cancelled`, which SHALL be a distinct
value from `done`. A state for work stalled on this board SHALL carry `blocked`, distinct from both
`todo` and `waiting`.

#### Scenario: The shipped workflow categorises its own states

- **WHEN** the shipped workflow is read
- **THEN** its `cancelled` state carries the category `cancelled`
- **AND** its `blocked` state carries the category `blocked`

### Requirement: A state's category does not decide whether it is terminal

Terminal-ness SHALL be decided by the state's terminal flag alone. No judgement about whether a task is
finished — the completion timestamp, the release of its dependents, what the claim path will hand out, or
which states satisfy a dependency — SHALL read the state's category.

#### Scenario: A cancelled state stays terminal

- **WHEN** a state carries the category `cancelled` and is declared terminal
- **THEN** it is reported among the workflow's terminal states
- **AND** a task moved into it is given a completion timestamp
- **AND** its dependents are released

#### Scenario: Recategorising does not change what a claim will hand out

- **WHEN** a state's category changes and its terminal flag does not
- **THEN** the claim path's judgement about that state is unchanged

### Requirement: A stored workflow keeps the categories it declared

The system SHALL NOT rewrite or re-derive the categories of a workflow that is already stored. A
workflow that declares a category SHALL be reported under that category, whatever the shipped workflow
declares for a similarly named state.

#### Scenario: A hand-authored workflow calling its cancelled state done is honoured

- **GIVEN** a stored workflow whose cancelled state declares the category `done`
- **WHEN** a task is moved into that state and the statistics are read
- **THEN** the task is counted in the `done` row
- **AND** it is not counted in the `cancelled` row

#### Scenario: An existing installation is not migrated

- **WHEN** an installation that already holds the shipped workflow starts
- **THEN** the stored workflow is left exactly as it is
- **AND** only a fresh installation receives the widened categories

### Requirement: Every category is drawn distinctly and degrades to its word

Every surface that colours a workflow state SHALL derive that colour from the state's category and from
nothing else. No two categories SHALL be drawn identically on a surface that draws colour at all. A
category the build does not recognise SHALL be drawn plainly rather than given a guessed colour.

Where a surface writes no colour, every category SHALL be drawn with no styling and SHALL remain
identifiable by its word.

#### Scenario: The six are mutually distinct on a colour terminal

- **WHEN** a board, a command-line listing or a browser page draws all six categories in colour
- **THEN** each is drawn in a style no other category is drawn in
- **AND** `cancelled` is not drawn in the style `done` is drawn in
- **AND** `blocked` is not drawn in the style `todo` is drawn in

#### Scenario: The terminal and the board agree

- **WHEN** a category is drawn on the board and on the command line
- **THEN** both write the same terminal colour for it

#### Scenario: A colourless terminal writes no attribute

- **WHEN** a board is drawn for a terminal that writes no colour
- **THEN** no category writes any escape sequence
- **AND** each column keeps the label naming its state

#### Scenario: An unrecognised category is not guessed at

- **WHEN** a state carries a category this build does not recognise
- **THEN** it is drawn in the ordinary text colour on every surface

### Requirement: A workflow key is addressable

A workflow key SHALL start with a letter, SHALL end with a letter or a digit, SHALL contain only letters,
digits, hyphens and underscores, and SHALL be at most 64 characters. This is the shape a project key already
holds, and it is held for the same reason: the key is spent as a path segment in a URL.

A key failing the shape SHALL be refused as invalid input, and the refusal SHALL name the workflow key so an
operator is not left guessing which field was wrong.

The rule SHALL be enforced by the contract rather than by one surface, so the command line, the HTTP API, the
browser editor and a snapshot import all refuse the same keys.

A refused key SHALL leave no workflow stored and SHALL produce no redirect.

#### Scenario: A key carrying a path separator

- **WHEN** a workflow is saved under a key containing a slash, or one made of relative path segments such as `../admin`
- **THEN** the save is refused as invalid input naming the workflow key
- **AND** no workflow is stored and the response carries no redirect location

#### Scenario: A key carrying a query or a fragment

- **WHEN** a workflow is saved under a key containing `?` or `#`
- **THEN** the save is refused as invalid input naming the workflow key

#### Scenario: A usable key is still usable

- **WHEN** a workflow is saved under a key of letters, digits, hyphens or underscores beginning with a letter
- **THEN** the save succeeds and the response redirects to that workflow's own route under the workflows prefix

#### Scenario: Every surface refuses it

- **WHEN** a workflow carrying such a key arrives through a snapshot import rather than through the editor
- **THEN** it is refused for the same reason
