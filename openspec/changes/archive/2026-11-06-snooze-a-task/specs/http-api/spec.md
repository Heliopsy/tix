## ADDED Requirements

### Requirement: Snooze over HTTP

`PUT /api/v1/tasks/{ref}/snooze` with `{"until": RFC3339}` SHALL snooze the task and return it,
`DELETE /api/v1/tasks/{ref}/snooze` SHALL unsnooze it and return it, and
`POST /api/v1/snoozes/wake` with an optional `{"limit": N}` SHALL run one wake pass and return
`{"woken": N}`. `GET /api/v1/tasks` SHALL take `snoozed=true|false|any`.

#### Scenario: Snooze and list

- **WHEN** a client snoozes OPS-3 over HTTP and lists tasks with `snoozed=true`
- **THEN** the listing holds OPS-3 with its `snoozed_until`
