## ADDED Requirements

### Requirement: Task creation

A task SHALL be creatable with a title, and optionally a body, project, priority, assignee, due date, labels, and custom field values. Only a title SHALL be required; every other attribute SHALL take a documented default.

#### Scenario: Create with only a title

- **WHEN** a task is created supplying only a title
- **THEN** the task is created in the default project, at the default priority, in its workflow's initial state, unassigned, with no due date and no labels

#### Scenario: Create with full attributes

- **WHEN** a task is created supplying a title, body, project, priority, assignee, due date, labels, and custom field values
- **THEN** all supplied attributes are stored and returned on the created task

#### Scenario: Empty title

- **WHEN** a task is created with an empty or whitespace-only title
- **THEN** the request is rejected with a validation error and no task is created

#### Scenario: Unknown project

- **WHEN** a task is created naming a project that does not exist in the tenant
- **THEN** the request is rejected with a not-found error and no task is created

### Requirement: Task identifiers and references

Every task SHALL have a stable opaque identifier and a human-readable reference composed of its project key and a per-project sequence number, such as `infra-42`. Both forms SHALL be accepted wherever a task reference is taken, and the reference SHALL be unique within its tenant.

#### Scenario: Reference assigned on create

- **WHEN** a task is created in the project with key `infra`
- **THEN** the task is returned with a stable identifier and a reference of the form `infra-<n>` where `<n>` is the next number in that project's sequence

#### Scenario: Operate by reference

- **WHEN** an operation is addressed to `infra-42`
- **THEN** it acts on the same task as the equivalent operation addressed to that task's stable identifier

#### Scenario: References are not reused

- **WHEN** a task is deleted and a new task is created in the same project
- **THEN** the new task receives a reference that has not previously been used in that project

#### Scenario: Unknown reference

- **WHEN** an operation is addressed to a reference that matches no task
- **THEN** the request is rejected with a not-found error

### Requirement: Task update

A task's attributes SHALL be updatable individually, and an update SHALL change only the attributes explicitly supplied.

#### Scenario: Partial update

- **WHEN** an update supplies a new assignee only
- **THEN** the assignee changes and the title, body, priority, due date, labels, and custom fields are unchanged

#### Scenario: Clearing an optional attribute

- **WHEN** an update explicitly clears the due date
- **THEN** the task is returned with no due date

#### Scenario: Invalid value in an update

- **WHEN** an update supplies a value that fails validation for its field
- **THEN** the whole update is rejected with a validation error and no attribute of the task is changed

### Requirement: Task deletion

A task SHALL support soft deletion, after which it is excluded from ordinary reads and lists but remains recoverable, and hard deletion, which removes it permanently.

#### Scenario: Soft delete

- **WHEN** a task is soft deleted
- **THEN** it no longer appears in list results and reading it without requesting deleted records reports it as not found

#### Scenario: Restore a soft-deleted task

- **WHEN** a soft-deleted task is restored
- **THEN** it appears in list results again with its previous attributes intact

#### Scenario: Hard delete

- **WHEN** a task is hard deleted
- **THEN** it is removed permanently and cannot be restored

#### Scenario: Delete a task with subtasks

- **WHEN** deletion is requested for a task that has child tasks
- **THEN** the request is rejected unless it explicitly requests that the children be deleted or detached, and the outcome is reported

### Requirement: Subtasks

A task MAY declare a parent task, expressing containment. A task SHALL have at most one parent, and a parent relationship SHALL NOT form a cycle.

#### Scenario: Attach a subtask

- **WHEN** a task is created or updated with a parent reference
- **THEN** the task is reported as a child of that parent and the parent lists it among its children

#### Scenario: Parent cycle

- **WHEN** a parent relationship is set that would make a task its own ancestor
- **THEN** the request is rejected with a validation error and no relationship is created

#### Scenario: Detach a subtask

- **WHEN** a task's parent reference is cleared
- **THEN** the task becomes a root task and no longer appears among the former parent's children

#### Scenario: Containment does not order work

- **WHEN** a parent task has children that are not in terminal states
- **THEN** the parent is not reported as blocked solely because of those children

### Requirement: Dependencies

A task MAY declare dependencies on other tasks, expressing ordering. Dependencies SHALL be distinct from the parent relationship, and a dependency graph SHALL NOT contain a cycle.

#### Scenario: Add a dependency

- **WHEN** task B is declared to depend on task A
- **THEN** A is listed among B's dependencies and B is listed among A's dependents

#### Scenario: Direct cycle

- **WHEN** a dependency is added that would make a task depend on itself, directly or through other tasks
- **THEN** the request is rejected with a validation error naming the cycle and no dependency is created

#### Scenario: Remove a dependency

- **WHEN** a dependency is removed
- **THEN** it no longer appears on either task and any blocked state caused solely by it is cleared

#### Scenario: Dependency on an unknown task

- **WHEN** a dependency is added naming a task that does not exist
- **THEN** the request is rejected with a not-found error

### Requirement: Blocked reporting

A task whose dependencies are not all in terminal states SHALL be reported as blocked. A task with no dependencies, or whose dependencies are all in terminal states, SHALL be reported as unblocked.

#### Scenario: Blocked by an open dependency

- **WHEN** a task depends on another task that is not in a terminal state
- **THEN** the task is reported as blocked and the unmet dependencies are reported with it

#### Scenario: Unblocked when dependencies complete

- **WHEN** every dependency of a blocked task reaches a terminal state
- **THEN** the task is reported as unblocked without any further action on it

#### Scenario: No dependencies

- **WHEN** a task has no dependencies
- **THEN** the task is reported as unblocked

### Requirement: Labels

Labels SHALL be attachable to and detachable from a task, a task SHALL carry any number of labels, and attaching a label already present SHALL be idempotent.

#### Scenario: Attach a label

- **WHEN** a label is attached to a task
- **THEN** the label appears on the task and the task appears when filtering by that label

#### Scenario: Attach a label twice

- **WHEN** a label already on a task is attached again
- **THEN** the operation succeeds and the label appears exactly once on the task

#### Scenario: Detach a label

- **WHEN** a label is detached from a task
- **THEN** the label no longer appears on the task and the task no longer matches a filter on that label

### Requirement: Comments

A task SHALL support comments, which can be created, edited, and soft deleted. A comment SHALL record its author and its creation time, and an edited comment SHALL record that it was edited.

#### Scenario: Add a comment

- **WHEN** a comment is added to a task
- **THEN** the comment is returned with the task's comment thread, attributed to its author, with its creation time

#### Scenario: Edit a comment

- **WHEN** a comment's body is edited
- **THEN** the new body is returned and the comment is marked as edited

#### Scenario: Soft delete a comment

- **WHEN** a comment is deleted
- **THEN** it is excluded from the task's comment thread and the remaining comments retain their order

#### Scenario: Comment on an unknown task

- **WHEN** a comment is added to a task reference that matches no task
- **THEN** the request is rejected with a not-found error

### Requirement: Artifacts

A task SHALL accept structured artifacts written by workers. An artifact SHALL carry a kind, a name, a JSON payload, and a content type, and MAY carry an inline blob. Artifacts on a task SHALL be listable and individually retrievable.

#### Scenario: Write an artifact

- **WHEN** an artifact is written to a task with a kind, name, JSON payload, and content type
- **THEN** the artifact is stored and appears when the task's artifacts are listed

#### Scenario: Retrieve an artifact payload

- **WHEN** a stored artifact is retrieved
- **THEN** its JSON payload and any inline blob are returned unchanged

#### Scenario: Invalid payload

- **WHEN** an artifact is written whose payload is not valid JSON
- **THEN** the request is rejected with a validation error and no artifact is stored

#### Scenario: Multiple artifacts of the same kind

- **WHEN** several artifacts of the same kind are written to one task
- **THEN** all of them are retained and listed, distinguished by name and creation time

### Requirement: Task filtering

Task lists SHALL be filterable by project, status, assignee, label, priority, due date, claimed state, blocked state, parent, free-text query, and custom field values. Multiple filters SHALL combine conjunctively.

#### Scenario: Single filter

- **WHEN** tasks are listed filtered by status
- **THEN** only tasks in that status are returned

#### Scenario: Combined filters

- **WHEN** tasks are listed filtered by project, label, and unblocked state together
- **THEN** only tasks satisfying all three conditions are returned

#### Scenario: Filter by custom field

- **WHEN** tasks are listed filtered by a custom field value
- **THEN** only tasks whose value for that field matches are returned

#### Scenario: Text query

- **WHEN** tasks are listed with a free-text query
- **THEN** tasks whose title or body matches the query are returned

#### Scenario: Unknown filter field

- **WHEN** tasks are listed with a filter naming a field that does not exist
- **THEN** the request is rejected with a validation error naming the unknown field

### Requirement: Task sorting

Task lists SHALL be sortable by documented fields including creation time, update time, priority, due date, and status, in ascending or descending order, with a deterministic total order.

#### Scenario: Sort by priority

- **WHEN** tasks are listed sorted by priority descending
- **THEN** tasks are returned in descending priority order

#### Scenario: Deterministic tie-breaking

- **WHEN** several tasks share the same value for the requested sort field
- **THEN** they are returned in a stable, deterministic order that is identical across repeated identical requests

#### Scenario: Unsupported sort field

- **WHEN** a sort is requested on a field that is not sortable
- **THEN** the request is rejected with a validation error

### Requirement: Keyset pagination

Every list result SHALL be paginated by an opaque cursor derived from the sort key and the task identifier. Offset-based pagination SHALL NOT be offered by any list operation.

#### Scenario: First page

- **WHEN** a list is requested with a page size and no cursor
- **THEN** at most that many results are returned together with a cursor for the next page when more results exist

#### Scenario: Subsequent page

- **WHEN** a list is requested with the cursor returned by the previous page
- **THEN** the results continue immediately after the last result of the previous page, with no duplicates and no gaps among unchanged rows

#### Scenario: Last page

- **WHEN** the final page of a list is returned
- **THEN** no next-page cursor is returned

#### Scenario: Offset is not accepted

- **WHEN** a list is requested with an offset parameter
- **THEN** the request is rejected rather than being served by skipping rows

#### Scenario: Cursor from a different query

- **WHEN** a cursor is supplied with filter or sort parameters that differ from those it was produced under
- **THEN** the request is rejected with a validation error

### Requirement: Optimistic concurrency

Every task SHALL carry a version that changes on each mutation. An update that supplies a version other than the task's current version SHALL be rejected as a conflict and SHALL NOT overwrite the stored task.

#### Scenario: Stale update rejected

- **WHEN** two updates are prepared from the same version and the first is applied
- **THEN** the second is rejected with a conflict error reporting the current version, and the task retains the result of the first update

#### Scenario: Current version accepted

- **WHEN** an update supplies the task's current version
- **THEN** the update is applied and the task's version changes

#### Scenario: Version omitted

- **WHEN** an update omits the version
- **THEN** the update is applied last-write-wins and the behaviour is documented as an explicit opt-out of conflict detection

### Requirement: Task tree view

A task and its descendants SHALL be retrievable as a tree, reporting each node's depth and its parent, so that containment can be rendered without repeated queries.

#### Scenario: Retrieve a tree

- **WHEN** the tree of a task with nested subtasks is requested
- **THEN** the task and all of its descendants are returned with their parent relationship and depth

#### Scenario: Leaf task

- **WHEN** the tree of a task with no children is requested
- **THEN** only that task is returned

#### Scenario: Depth limit

- **WHEN** a tree is requested with a maximum depth
- **THEN** only nodes within that depth are returned and nodes with omitted children are marked as having more

#### Scenario: Deleted descendants

- **WHEN** the tree of a task with soft-deleted descendants is requested without asking for deleted records
- **THEN** the soft-deleted nodes and their subtrees are omitted
