## ADDED Requirements

### Requirement: The project PATCH changes the key

`PATCH /api/v1/projects/{ref}` SHALL accept a `key` member that changes the project's key, answering
with the updated project. A key held by another project SHALL be answered with 409 and an invalid key
with 400. A path naming a project or task by a kept alias SHALL be served as if it named the current
key, and the response SHALL carry the current key and reference.

#### Scenario: Renaming over the API

- **WHEN** a client sends `PATCH /api/v1/projects/infra` with `{"key":"ops"}`
- **THEN** the response is 200 with `"key":"ops"`

#### Scenario: Reading by an old reference

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** a client sends `GET /api/v1/tasks/infra-8`
- **THEN** the response is 200 with `"ref":"ops-8"`
