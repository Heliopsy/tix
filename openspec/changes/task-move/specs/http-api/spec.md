## ADDED Requirements

### Requirement: Move endpoint

The API SHALL expose `POST /api/v1/tasks/{ref}/move` taking `{"project_ref": KEY}` and answering `200` with
the moved task and `from_ref`, `from_project`, `project`, `from_status`, `status_kept` and `dropped_fields`.

#### Scenario: Move over HTTP

- **WHEN** a client sends `POST /api/v1/tasks/infra-4/move` with `{"project_ref": "web"}`
- **THEN** the response is 200, `task.ref` is a WEB reference and `from_ref` is `infra-4`

### Requirement: Bulk move to a project

`POST /api/v1/tasks/bulk` SHALL accept the action `move_project` with `project_ref`, moving each task as the
move endpoint does and naming each new reference in the outcome's `moved_ref`.

#### Scenario: Bulk move

- **WHEN** a client bulk moves infra-1 and infra-999 to OPS
- **THEN** infra-1 is applied with a `moved_ref` in OPS and infra-999 is refused as not found
