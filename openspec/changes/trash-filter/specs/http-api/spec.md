## ADDED Requirements

### Requirement: The task listing takes a trashed parameter

`GET /api/v1/tasks` SHALL accept `trashed=true`, listing only soft-deleted tasks, and `trashed=false`,
listing only live ones. The client SHALL send the filter contract's `Trashed` field as that parameter.

#### Scenario: Asking for the trash over the API

- **GIVEN** a deleted task and a live one
- **WHEN** a client lists tasks with `trashed=true`
- **THEN** only the deleted task is returned
