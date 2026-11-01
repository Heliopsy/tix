## ADDED Requirements

### Requirement: The browser follows the tenant's trash

The Tenant screen SHALL offer the trash setting with an explanation and, when the trash holds tasks, how
many. With the trash off, the sidebar SHALL NOT carry a Trash entry, the trash address SHALL show the
refusal in the filter bar, and a delete, single or bulk, that has not been confirmed SHALL answer with a
confirmation that names the tasks and says the delete cannot be undone, deleting nothing. A confirmed
delete SHALL NOT offer Undo. The confirmation SHALL be decided by the delete handlers so it applies to
every place a Delete is drawn.

#### Scenario: Deleting with the trash off

- **GIVEN** a tenant whose trash is off
- **WHEN** the reader deletes a task
- **THEN** a confirmation is drawn and the task still exists, and confirming deletes it with no Undo

#### Scenario: The trash leaves the sidebar

- **GIVEN** a tenant whose trash is off
- **WHEN** the reader opens the task list
- **THEN** the sidebar has no Trash entry
