## ADDED Requirements

### Requirement: A trashed task can be purged

Purging a task in the trash SHALL permanently delete it and its trashed descendants, deepest first, exactly as the trash rule does, writing per task one `task.delete` audit entry and one `task.deleted` event whose payload sets `hard` and `purged`, all in one transaction. It SHALL need the permission and scope a delete needs.

#### Scenario: Purging a trashed task with a trashed subtask

- **GIVEN** a task deleted with its subtask, both in the trash
- **WHEN** the task is purged
- **THEN** neither can be found even with deleted tasks included, and two `task.delete` entries record it

#### Scenario: A reader without delete permission

- **WHEN** a viewer purges a trashed task
- **THEN** the call is refused as forbidden and the task stays in the trash

### Requirement: Only a trashed task can be purged

Purging a live task SHALL be refused as a failed precondition whose detail `reason` is `not_trashed`, leaving the task untouched. With the tenant's trash off, purging SHALL be refused with reason `trash_off`, so the trash keeps what it held until it is turned on again. No retention rule forbids an early purge: the trash rule is a maximum age.

#### Scenario: Purging a live task

- **WHEN** a live task is purged
- **THEN** it is refused with reason `not_trashed` and the task is still live

#### Scenario: Purging with the trash off

- **GIVEN** a task in the trash and then the trash turned off
- **WHEN** the task is purged
- **THEN** it is refused with reason `trash_off` and the task is still held
