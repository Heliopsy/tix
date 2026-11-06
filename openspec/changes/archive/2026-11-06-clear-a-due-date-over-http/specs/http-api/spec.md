## ADDED Requirements

### Requirement: A task update tells an absent due date from a null one

`PATCH /api/v1/tasks/{ref}` SHALL treat `due_at` in three ways: an absent key SHALL leave the due date
unchanged, `null` SHALL clear it, and an RFC 3339 timestamp SHALL set it. The OpenAPI document SHALL
describe the field as nullable.

#### Scenario: Null clears the due date

- **WHEN** a task with a due date is updated with `{"due_at": null}`
- **THEN** the task read back has no `due_at`

#### Scenario: An absent key leaves it

- **WHEN** a task with a due date is updated with `{"title": "renamed"}`
- **THEN** the task read back keeps its due date

#### Scenario: A remote client clears it

- **WHEN** `tix task edit REF --due ""` runs against `--server`
- **THEN** the server clears the task's due date
