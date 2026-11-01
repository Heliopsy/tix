## ADDED Requirements

### Requirement: The API sets and reports the trash

`PATCH /api/v1/tenants/{ref}` SHALL accept `trash_off`. `GET /api/v1/trash` SHALL answer `enabled` and
`held`. A refusal because the trash is off SHALL be `422 precondition_failed` with `details.reason`
`trash_off`.

#### Scenario: Restoring over the API with the trash off

- **GIVEN** a task in the trash and the trash turned off
- **WHEN** a client posts to the task's restore route
- **THEN** it answers 422 with `details.reason` `trash_off`
