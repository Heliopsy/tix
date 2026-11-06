## ADDED Requirements

### Requirement: Copy endpoint

The API SHALL expose `POST /v1/tasks/{ref}/copy` taking a JSON body with optional `project_ref`,
`title`, `keep_status`, `with_comments`, `with_subtasks`, `with_dependencies` and `link`. It
SHALL answer `201 Created` with the root copy. A task read SHALL carry `copied_from_id` when the
task records one.

#### Scenario: Copy over HTTP

- **WHEN** a client sends `POST /v1/tasks/OPS-3/copy` with `{"project_ref": "WEB", "link": true}`
- **THEN** the response is 201 and its body is a WEB task whose `copied_from_id` is OPS-3's identifier

#### Scenario: Unknown target

- **WHEN** a client copies to a project that does not exist
- **THEN** the response is 404
