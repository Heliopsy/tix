## ADDED Requirements

### Requirement: Moving a task keeps the task

The service SHALL move a task into another project as the same task: its identifier, comments, artifacts,
history, dependencies, stars, tags, assignee and parent SHALL be kept, and it SHALL take the target's next
number. The move, its audit entry and its events SHALL be one transaction. Subtasks SHALL stay where they are.

#### Scenario: Renumbered, same task

- **WHEN** INFRA-4 with a comment moves to OPS, whose last task is OPS-1
- **THEN** the task is OPS-2 with the same identifier and its comment

### Requirement: A moved task's old reference resolves

The store SHALL keep the project and number a task had before each move, and resolving a reference SHALL
fall back to them, so the old reference reaches the task on every surface. A project SHALL never reissue a
number a task held.

#### Scenario: Old reference after a move

- **WHEN** INFRA-4 has moved to OPS-2 and a client reads INFRA-4
- **THEN** it gets the task whose reference is OPS-2

#### Scenario: Number not reissued

- **WHEN** a task is created in INFRA after INFRA-4 moved away
- **THEN** it does not get INFRA-4

### Requirement: Status on a move

A moved task SHALL keep its status when the target workflow has a state of the same key, and SHALL
otherwise start in the target's initial state. The answer SHALL say which through `status_kept`, and the
completion time SHALL follow whether the new state is terminal.

#### Scenario: Target lacks the state

- **WHEN** a task in `done` moves to a project whose workflow has no `done`
- **THEN** it is in the target's initial state, `status_kept` is false and it is not completed

### Requirement: Custom fields on a move

A moved task SHALL keep the custom field values the target defines and accepts, drop the others and name
them in `dropped_fields`, then take the target's defaults. A required target field the task cannot fill
SHALL refuse the move as invalid.

#### Scenario: Field the target lacks

- **WHEN** a task with `env` and `team` moves to a project that defines only `team`
- **THEN** it keeps `team`, loses `env`, and `dropped_fields` is `["env"]`

### Requirement: Who may move a task

A move SHALL need `task.update` on the task and `task.create` in the target. A task claimed by another actor
under a live lease SHALL be refused as a conflict; the caller's own claim SHALL stay on the task. A move to
the task's own project SHALL be invalid, and to an archived project a failed precondition.

#### Scenario: Claimed by somebody else

- **WHEN** a worker holds a live claim on a task and another actor moves it
- **THEN** the move is refused as a conflict and nothing is written

#### Scenario: Viewer

- **WHEN** a read-only actor moves a task
- **THEN** the move is forbidden

### Requirement: A move is recorded in both projects

A move SHALL write one `task.move` audit entry whose payload names `from_ref`, `ref`, `from_project`,
`project`, `from_status` and `status`, and SHALL emit `task.updated` with that payload in both the source
and the target project.

#### Scenario: Boards on both sides

- **WHEN** a task moves from INFRA to OPS
- **THEN** one `task.updated` event is emitted for each of the two projects
