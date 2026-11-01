## ADDED Requirements

### Requirement: Bulk task actions

The service SHALL offer `BulkTasks` taking an action, a list of task references and the
action's argument, and SHALL apply the action to each referenced task through the same rules as
the single-task operation: move to a status (along the shortest route of the task's own
workflow), set the priority, add a tag, remove a tag, set or clear the assignee, star, unstar,
copy to a project, soft delete and restore. Each task SHALL be changed in its own transaction
with its own audit entry and event. The answer SHALL carry one outcome per distinct reference,
in the order given, saying whether it was applied and, when refused, the error code and
message; a copy's outcome SHALL name the new task. A refusal for one task SHALL NOT undo or
prevent the change to another.

#### Scenario: Partial refusal

- **WHEN** a caller moves OPS-1 and OPS-2 to `done` and OPS-2's workflow has no route to `done`
- **THEN** OPS-1 is `done`, OPS-2 is unchanged, and the outcomes report OPS-1 applied and OPS-2 refused with the reason

#### Scenario: One audit entry per task

- **WHEN** a caller sets the priority of three tasks in one bulk action
- **THEN** the audit log holds a `task.update` entry for each of the three tasks and three `task.updated` events are emitted

### Requirement: Bulk action limits and authority

A bulk action SHALL name a known action, between 1 and 500 references and the argument the
action needs, and SHALL be refused as invalid otherwise. A reference named twice SHALL be acted
on once. Before acting on any task, the service SHALL check the caller holds the scope the
action needs and SHALL refuse the whole request when it does not.

#### Scenario: Too many references

- **WHEN** a caller sends a bulk action naming 501 references
- **THEN** the request is refused as invalid and no task changes

#### Scenario: Missing scope

- **WHEN** a caller holding only `task:read` asks for a bulk delete
- **THEN** the request is refused as forbidden and no task changes
