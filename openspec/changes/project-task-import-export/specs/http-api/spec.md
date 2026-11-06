## ADDED Requirements

### Requirement: Project task export endpoint

The API SHALL expose `POST /api/v1/projects/{ref}/tasks/export` taking the export choices as a JSON body,
and SHALL answer with the file as `text/csv`, `application/json`, `application/x-ndjson` or
`text/markdown` with a `Content-Disposition` naming it, or with the error envelope when refused before
the first byte.

#### Scenario: JSON export over HTTP

- **WHEN** a client posts `{"format":"json","columns":["title"]}` to the export route of INFRA
- **THEN** the response is 200 `application/json`, an array of objects with a `title` key, named `infra-tasks.json`

#### Scenario: Refused export

- **WHEN** a client posts an export naming an unknown column
- **THEN** the response is 400 in the error envelope with no `Content-Disposition`
