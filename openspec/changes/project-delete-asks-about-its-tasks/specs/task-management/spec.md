## ADDED Requirements

### Requirement: Deleting a project decides what happens to its tasks

Deleting a project that holds tasks, live or soft deleted, SHALL require the caller to choose between moving the
tasks to another project and deleting them with the project. A deletion of such a project that makes neither choice
SHALL be refused as a failed precondition naming the number of tasks, and SHALL write nothing. Making both choices,
or naming the project itself as the target, SHALL be refused as invalid input. Naming an archived project as the
target SHALL be refused as a failed precondition. A project holding no tasks SHALL delete without a choice.

Deleting a project SHALL remain permanent. When the tasks are deleted with it, every one of them, soft deleted
tasks included, SHALL be removed permanently together with its comments, attachments, tags, dependencies and stars,
and each SHALL be recorded with its own `task.delete` audit entry and `task.deleted` event marked hard, in the same
transaction as the project's own audit entry and event.

Moving and deleting with the project SHALL require the authority project deletion requires. Moving SHALL in
addition require the authority to update tasks in the target project.

#### Scenario: A project with tasks and no choice is refused

- **GIVEN** a project holding three tasks
- **WHEN** it is deleted without a choice
- **THEN** the deletion fails as a precondition naming three tasks
- **AND** the project and its tasks are unchanged

#### Scenario: An empty project needs no choice

- **WHEN** a project holding no tasks is deleted without a choice
- **THEN** the project is removed with one audit entry and one event

#### Scenario: Deleting the tasks with the project records each one

- **GIVEN** a project holding two live tasks and one soft deleted task
- **WHEN** it is deleted with its tasks
- **THEN** the project and all three tasks are gone
- **AND** three `task.deleted` events marked hard and one `project.deleted` event were written in one transaction

#### Scenario: Both choices at once are invalid

- **WHEN** a deletion names a target project and also asks to delete the tasks
- **THEN** it is refused as invalid input and nothing is written

#### Scenario: Moving needs task authority on the target

- **GIVEN** a caller holding project write but not task write on the target project
- **WHEN** the caller deletes a project moving its tasks to that target
- **THEN** the deletion is refused as permission denied and nothing is written

### Requirement: Moving a deleted project's tasks keeps everything that belongs to them

Moving the tasks of a deleted project SHALL give each task, in its original sequence order, the next number the
target project's own counter issues, and SHALL keep the task's identifier, so that its history, comments,
attachments, tags, dependencies, stars, parent, repeat rule, lease and soft deletion are kept unchanged. A tag scoped
to the deleted project that a moved task carries SHALL be re-created under the target project with the same name
and colour and stay on the task.

Each moved task's status SHALL be mapped onto the target project's workflow: to the same state when the target
workflow defines it, otherwise to the first state of the target workflow, in declared order, of the same category as
the task's state, otherwise to the target workflow's initial state.

Each moved task SHALL be recorded with a `task.move` audit entry and a `task.updated` event naming its previous and
new reference, project and status, in the same transaction as the project's deletion.

#### Scenario: Tasks are renumbered in the target

- **GIVEN** a target project whose counter has issued 4
- **WHEN** a project holding tasks numbered 1 and 2 is deleted moving them to the target
- **THEN** they are numbered 5 and 6 in the target, in that order

#### Scenario: A moved task keeps its comments, tags, stars and dependencies

- **GIVEN** a task with a comment, a tag, a star and a dependency on a task in a third project
- **WHEN** its project is deleted moving it elsewhere
- **THEN** the task under its new reference still has the comment, the tag, the star and the dependency

#### Scenario: A state the target lacks maps by category

- **GIVEN** a task in `review`, a state of category in progress, and a target workflow without `review` whose first
  in progress state is `doing`
- **WHEN** its project is deleted moving it to that target
- **THEN** the task is in `doing`

#### Scenario: A soft deleted task stays restorable

- **WHEN** a project holding a soft deleted task is deleted moving its tasks
- **THEN** the task is soft deleted in the target and restoring it under its new reference succeeds

### Requirement: A project deletion can be planned before it is made

The service SHALL report, without writing, what deleting a project would do: the number of live and of soft deleted
tasks it holds, the target named, and for a move the status mapping with the number of tasks in each mapped status.
The plan SHALL apply the same validation of the choice and the target as the deletion and SHALL require the authority
to read projects, and the authority to update tasks in the target when one is named.

#### Scenario: The plan states the mapping

- **GIVEN** a project with two tasks in `todo` and one in `review`, and a target workflow without `review`
- **WHEN** the deletion is planned moving to that target
- **THEN** the plan states `todo` to `todo` for two tasks and `review` to the mapped state for one task
- **AND** nothing is written
