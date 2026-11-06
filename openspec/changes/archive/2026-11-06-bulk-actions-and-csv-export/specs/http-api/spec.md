## ADDED Requirements

### Requirement: Bulk endpoint

The API SHALL expose `POST /v1/tasks/bulk` taking a JSON body with `action`, `refs` and the
action's argument (`to`, `priority`, `tag`, `assignee_actor_id`, `project_ref`, `cascade`), and
SHALL answer `200 OK` with the per-task outcomes and the applied and refused counts, also when
some tasks were refused.

#### Scenario: Bulk over HTTP

- **WHEN** a client sends `POST /v1/tasks/bulk` with `{"action": "priority", "refs": ["OPS-1", "OPS-2"], "priority": 1}`
- **THEN** the response is 200, both outcomes are applied, and both tasks have priority 1

#### Scenario: Unknown action

- **WHEN** a client sends a bulk body whose action is `explode`
- **THEN** the response is 400

### Requirement: Task listing as CSV

`GET /v1/tasks` SHALL answer with `text/csv` when the request carries `format=csv` or an
`Accept` header naming `text/csv`. The body SHALL hold a header row and one row per task of the
page, in the columns `ref, project, title, status, priority, assignee, tags, due, created,
updated`, escaped by RFC 4180, with any cell starting with `=`, `+`, `-`, `@`, a tab or a
carriage return prefixed with `'`. The next cursor, when there is one, SHALL be named in the
`X-Tix-Next-Cursor` response header.

#### Scenario: CSV by query parameter

- **WHEN** a client requests `GET /v1/tasks?format=csv`
- **THEN** the response has content type `text/csv` and its first line is the header row

#### Scenario: Formula guard

- **WHEN** a task titled `=HYPERLINK("x")` is listed as CSV
- **THEN** its title cell reads `'=HYPERLINK("x")`, quoted
