## ADDED Requirements

### Requirement: A task moves to another project

A task SHALL move to another project of the same tenant, keeping its identifier, comments, attachments,
stars, dependencies, custom field values, lease and history. It SHALL take the next number the target's
counter issues and its status SHALL be mapped as a deleted project's moved tasks are. A tag scoped to the
source project that it carries SHALL be re-created under the target and stay on the task.

#### Scenario: Renumbered and mapped

- **GIVEN** OPS-3 in `review`, and project WEB whose counter has issued 4 and whose workflow has no `review` but has `testing` in the same category
- **WHEN** OPS-3 is moved to WEB
- **THEN** it is WEB-5 in `testing`, with the same identifier and comments

#### Scenario: A scoped tag follows the task

- **WHEN** a task carrying a tag scoped to its project is moved
- **THEN** the task carries a tag of that name and colour scoped to the target

### Requirement: A moved task takes its subtree and leaves its parent

Moving a task SHALL move its live descendants that sit in the same project, in pre-order after it, each
keeping its parent link and taking its own new number. A moved task that has a parent SHALL be detached
from it and become a root task in the target.

#### Scenario: Subtasks follow

- **GIVEN** OPS-3 with subtask OPS-4, which has subtask OPS-5
- **WHEN** OPS-3 is moved to WEB
- **THEN** all three are in WEB, the new OPS-5 is a subtask of the new OPS-4 and that of the new OPS-3

#### Scenario: A subtask moved alone

- **GIVEN** OPS-4 is a subtask of OPS-3
- **WHEN** OPS-4 is moved to WEB
- **THEN** it is a root task in WEB and OPS-3 has no children

### Requirement: Moving a task is refused where it cannot land

A move SHALL be refused as invalid when the target is the task's own project, as a failed precondition
when the target is archived, as not found when the target does not exist in the tenant, as permission
denied without task update authority on every moved task and on the target, and as a conflict when a
given version is stale. A refused move SHALL write nothing.

#### Scenario: Stale version

- **WHEN** a task at version 3 is moved with version 2
- **THEN** the move is refused as a conflict and the task stays in its project

#### Scenario: No authority on the target

- **GIVEN** a caller pinned to project OPS
- **WHEN** it moves an OPS task to WEB
- **THEN** the move is refused as permission denied
