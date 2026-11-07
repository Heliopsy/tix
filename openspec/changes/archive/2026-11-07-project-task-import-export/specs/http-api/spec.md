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

### Requirement: Project task import endpoint

The API SHALL expose `POST /api/v1/projects/{ref}/tasks/import` taking the import choices as JSON with the
file as `content`, or as a multipart form with the file in a `file` part, and SHALL answer with the result,
`committed` false when any row is in error.

#### Scenario: Multipart import

- **WHEN** a client posts a multipart form with a CSV in `file` and a mapping field
- **THEN** the response is 200 with the tasks created and `committed` true

### Requirement: Import upload limit

The server SHALL cap the body of a project task import, over the API and in the browser's upload step, at `server.max_import_bytes`, 32 MiB by default, in place of the general body limit, which SHALL still cap every other request. A larger import SHALL be answered with 413 and a message naming the limit and the key.

#### Scenario: An import larger than the general limit

- **WHEN** a client uploads a 5 MiB file to the import endpoint of a server with a 1 MiB body limit
- **THEN** the import is read and answered with 200

#### Scenario: An import over its own limit

- **WHEN** a client uploads a file larger than `server.max_import_bytes`
- **THEN** the response is 413 with a message naming the limit and `server.max_import_bytes`

#### Scenario: Other routes keep the general limit

- **WHEN** a client posts a body larger than the general limit to any route that is not an import
- **THEN** the response is 413
