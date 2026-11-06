## ADDED Requirements

### Requirement: Template events

Saving, editing and deleting a template SHALL emit `template.updated` with the template's
identifier and name and the audit action in the payload. Using one SHALL emit `task.created`
per new task with the template's name in the payload.

#### Scenario: Delete emits

- **WHEN** a member deletes template "release"
- **THEN** one `template.updated` event with action `template.delete` is recorded
