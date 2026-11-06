## ADDED Requirements

### Requirement: Template endpoints

The API SHALL expose `GET /v1/templates` (with `project`, `limit` and `cursor`),
`POST /v1/templates` taking `name`, `task_ref`, `tenant_wide`, `with_subtasks` and `due_in`,
`GET`, `PATCH` and `DELETE /v1/templates/{ref}`, and `POST /v1/templates/{ref}/use` taking
`project_ref`, `title` and `due_in`, answering `201 Created` with the new root task.

#### Scenario: Use over HTTP

- **WHEN** a client posts `{"due_in": "3d"}` to `/v1/templates/release/use`
- **THEN** the response is 201 and its body is the new task with a due date

#### Scenario: Unknown template

- **WHEN** a client reads `/v1/templates/nope`
- **THEN** the response is 404
