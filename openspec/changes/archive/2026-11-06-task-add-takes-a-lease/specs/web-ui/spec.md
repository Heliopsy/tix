## ADDED Requirements

### Requirement: The new task form sets a lease

The browser's Add with details form SHALL carry a Lease field under More options, blank to inherit. A lease above
the maximum SHALL draw the form again with the refusal on that field and create nothing.

#### Scenario: Adding a task with a lease in the browser

- **WHEN** a writer creates a task with 2h in the Lease field
- **THEN** the task screen's Lease row holds 2h and says claims last 2h, set on this task
