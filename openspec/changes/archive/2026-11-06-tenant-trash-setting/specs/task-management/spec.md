## ADDED Requirements

### Requirement: Deletes obey the tenant's trash

With the tenant's trash off, deleting a task SHALL remove it permanently whatever the caller asked,
restoring a task SHALL be refused, and listing only deleted tasks SHALL be refused, each refusal a
precondition failure whose detail `reason` is `trash_off`. Listing that merely includes deleted tasks
SHALL still be answered. Any reader SHALL be able to ask whether the trash is on and how many tasks it
holds.

#### Scenario: A delete with the trash off is permanent

- **GIVEN** a tenant whose trash is off
- **WHEN** a task is deleted without asking for a hard delete
- **THEN** the task cannot be listed even with deleted tasks included, and cannot be restored

#### Scenario: Restore and the trash listing are refused

- **GIVEN** a task in the trash and then the trash turned off
- **WHEN** the task is restored, or tasks are listed with `is:trashed`
- **THEN** both are refused as a failed precondition with reason `trash_off`
