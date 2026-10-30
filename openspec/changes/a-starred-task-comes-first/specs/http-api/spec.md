## ADDED Requirements

### Requirement: Star endpoints

The API SHALL expose `PUT /v1/tasks/{ref}/star` to star a task and `DELETE /v1/tasks/{ref}/star`
to unstar it, for the authenticated actor. Both SHALL answer `204 No Content` whether or not the
state changed. A task read SHALL carry `"starred": true` when the reading actor has starred it and
SHALL omit the field otherwise. The task list SHALL accept `starred=true` and `starred=false`.

#### Scenario: Star over HTTP

- **WHEN** a client sends `PUT /v1/tasks/OPS-1/star`
- **THEN** the response is 204 and a following `GET /v1/tasks/OPS-1` carries `"starred": true`

#### Scenario: Unstar over HTTP is idempotent

- **WHEN** a client sends `DELETE /v1/tasks/OPS-1/star` for a task it has not starred
- **THEN** the response is 204
