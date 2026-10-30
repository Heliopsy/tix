## ADDED Requirements

### Requirement: Recurrence on the task resource

A task resource SHALL carry `recurrence`, the canonical rule, and `recurrence_next_id`, the successor's
identifier, each omitted when empty. Task creation SHALL accept `recurrence`. A task update SHALL accept
`recurrence`, where an empty string removes the rule and an absent key leaves it unchanged. A rule that does
not parse SHALL be refused with the validation error envelope. `recurrence_next_id` SHALL NOT be accepted
as input.

#### Scenario: Create

- **WHEN** a task is created with `"recurrence": "daily"`
- **THEN** the response carries `"recurrence": "every day"`

#### Scenario: Invalid rule

- **WHEN** an update carries `"recurrence": "every blue moon"`
- **THEN** the response is a 400 validation error and the task is unchanged
