## ADDED Requirements

### Requirement: A task read carries its checklist count

A task in any read or update response SHALL carry `checklist` with `done` and `total` when its
description has GitHub task-list items, and SHALL omit the field when it has none. The count SHALL be
computed from the body on read and SHALL NOT appear in audit snapshots or exports.

#### Scenario: The count travels with the task

- **GIVEN** a task whose body is `- [x] a\n- [ ] b`
- **WHEN** it is read
- **THEN** the response carries `"checklist": {"done": 1, "total": 2}`

### Requirement: The API ticks one checklist item

`POST /api/v1/tasks/{ref}/checklist` SHALL take `item`, counted from one in reading order, `done`, and an
optional `version`, and SHALL require `task:write`. It SHALL rewrite only that item's marker, record a
`task.update` audit entry and event in the same transaction, and answer with the task. A stale version
SHALL answer `409`; an item number the description does not have SHALL answer `400`; an item already in
the asked-for state SHALL answer `200` and write nothing.

#### Scenario: A stale tick is refused

- **WHEN** a tick names a version the task has moved past
- **THEN** the answer is `409` and the body is unchanged

### Requirement: The task listing filters by checklist

`GET /api/v1/tasks` SHALL accept `checklist=true|false`, selecting tasks whose body does or does not
carry a checklist marker, and `checklist_open=true|false`, selecting those that do or do not carry an
unticked one.

#### Scenario: Only open checklists

- **WHEN** a listing asks for `checklist_open=true`
- **THEN** only tasks with an unticked marker are listed
