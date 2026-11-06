## ADDED Requirements

### Requirement: The API takes keys and references in any case and answers the stored value

Every API route and query parameter that names a project or a task SHALL accept it in any case.
Responses, webhook payloads and events SHALL carry the stored lower-case key and reference, unchanged
from earlier releases.

#### Scenario: Upper-case reference

- **WHEN** a client requests `GET /api/v1/tasks/INFRA-1`
- **THEN** the answer is the task with `"ref": "infra-1"`

#### Scenario: Upper-case project filter

- **WHEN** a client requests `GET /api/v1/tasks?project=INFRA`
- **THEN** the tasks of `infra` are listed
