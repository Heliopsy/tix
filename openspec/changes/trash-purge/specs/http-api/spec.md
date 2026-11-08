## ADDED Requirements

### Requirement: The API purges a trashed task

The server SHALL serve `DELETE /api/v1/tasks/{ref}/trash` (`task.purge`), answering `204` once the task and its trashed subtasks are permanently deleted, `412` with reason `not_trashed` for a live task or `trash_off` when the trash is off, `404` for an unknown task and `403` without `task:delete`.

#### Scenario: Purging over HTTP

- **GIVEN** a task in the trash
- **WHEN** a client sends DELETE to its trash route
- **THEN** the server answers 204 and the task is gone for good
