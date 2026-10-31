## ADDED Requirements

### Requirement: The project deletion route takes the task choice and the plan has a route

`DELETE /api/v1/projects/{ref}` SHALL accept the query parameters `move_tasks_to` and `delete_tasks`, SHALL answer
a project holding tasks and given neither with the precondition error envelope, and SHALL answer a successful
deletion with `200` and the deletion's result. `GET /api/v1/projects/{ref}/deletion` SHALL accept the same
parameters and return the plan without writing.

#### Scenario: Deleting with no choice over HTTP

- **WHEN** a project holding tasks is deleted over HTTP without a parameter
- **THEN** the response is the precondition error and the project remains

#### Scenario: Planning over HTTP

- **WHEN** `GET /api/v1/projects/{ref}/deletion?move_tasks_to=other` is requested
- **THEN** the response carries the task counts and the status mapping
