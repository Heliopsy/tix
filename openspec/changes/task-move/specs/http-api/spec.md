## ADDED Requirements

### Requirement: Move endpoint

The API SHALL expose `POST /api/v1/tasks/{ref}/move` taking a JSON body with a required `project_ref`
and an optional `version`, and SHALL answer `200 OK` with the moved task under its new reference.

#### Scenario: Move over HTTP

- **WHEN** a client sends `POST /api/v1/tasks/OPS-3/move` with `{"project_ref": "WEB"}`
- **THEN** the response is 200 and its body is a WEB task with OPS-3's identifier

#### Scenario: Stale version over HTTP

- **WHEN** the body carries a version older than the task's
- **THEN** the response is 409
