## ADDED Requirements

### Requirement: A task's lease can be set at creation

Creating a task SHALL accept the task's own lease. A lease longer than the ceiling the task's project is held to
SHALL be refused as invalid input and no task SHALL be created. A claim naming no TTL on a task created with a
lease SHALL be granted that lease with source `task`.

#### Scenario: Creating a task with a lease

- **WHEN** a task is created with a 2h lease and later claimed without a TTL
- **THEN** the claim's lease is 2h from the task

#### Scenario: A lease above the maximum is refused at creation

- **WHEN** a task is created with a lease longer than its project's effective maximum
- **THEN** the create is refused as invalid input and no task exists
