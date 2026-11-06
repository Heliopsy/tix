## ADDED Requirements

### Requirement: Quick-add over HTTP

`POST /v1/tasks` SHALL accept `"parse": true`, which reads `title` through the quick-add
parser, resolving relative dates in UTC, before the task is created. Without it the title
SHALL be stored literally. A field in the body that the line contradicts SHALL be refused
with 400.

#### Scenario: Parsed create

- **WHEN** a client posts `{"title": "Fix login !high #auth", "parse": true}`
- **THEN** the response is 201 and the task is titled `Fix login` with priority high and the tag `auth`

#### Scenario: Unknown project

- **WHEN** a client posts `{"title": "Fix login +nope", "parse": true}` and no project has that key
- **THEN** the response is 404 and no task is created
