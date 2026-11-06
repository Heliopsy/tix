## ADDED Requirements

### Requirement: The API sets a lease when it creates a task

`POST /api/v1/tasks` SHALL accept `lease`, a duration such as `"4h"`. A lease above the effective maximum SHALL be
refused with `invalid` and create nothing.

#### Scenario: Creating a task with a lease over HTTP

- **WHEN** a client posts a task with `"lease": "2h"`
- **THEN** the created task carries a two-hour lease
