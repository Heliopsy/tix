## ADDED Requirements

### Requirement: Housekeeping policy per tenant and per project

Each tenant SHALL hold a housekeeping policy with three rules, each off or a number of days: moving tasks in a
terminal done-category state to the trash, the same for terminal cancelled-category states, and permanently
deleting tasks that have been in the trash longer. Each project MAY override each rule as inherit, off or a
number of days. Every rule SHALL be off unless set. Only a tenant admin MAY read or change the policy, and a
change SHALL write one audit entry.

#### Scenario: Nothing changes by default

- **GIVEN** a tenant that never set a housekeeping policy, with tasks done a year ago
- **WHEN** the pruner ticks
- **THEN** no task is moved or deleted

#### Scenario: A project overrides the tenant

- **GIVEN** the tenant default moves done tasks after 30 days and project OPS sets it off
- **WHEN** the pruner ticks with done tasks 40 days old in OPS and in another project
- **THEN** only the other project's task is in the trash

### Requirement: Finished tasks only ever go to the trash

Housekeeping SHALL move finished tasks by soft deleting them, and SHALL NOT move a task with a live child.
Turning on a finished-task rule SHALL be refused as a failed precondition with reason `trash_off` while the
tenant's trash is off, and with the trash off every rule SHALL pause.

#### Scenario: Trash off refuses the rule

- **GIVEN** a tenant whose trash is off
- **WHEN** an admin sets the done rule to 30 days
- **THEN** it is refused with reason `trash_off`

#### Scenario: Trash turned off later pauses the rules

- **GIVEN** a done rule of 30 days and then the trash turned off
- **WHEN** the pruner ticks
- **THEN** no task is deleted and the preview reports the policy as paused

### Requirement: The trash empties itself

With a trash rule of N days, the pruner SHALL permanently delete tasks soft deleted more than N days ago,
each with its trashed and expired descendants, deepest first.

#### Scenario: Old trash is purged

- **GIVEN** a trash rule of 30 days, a task deleted 40 days ago and one deleted yesterday
- **WHEN** the pruner ticks
- **THEN** the first is gone for good and the second is still in the trash

### Requirement: Housekeeping is bounded, attributed and safe to repeat

Each tick SHALL act on at most the configured limit per rule, SHALL write one `task.delete` audit entry and one
`task.deleted` event per task under the system actor, and SHALL skip a task another worker has already acted
on without recording it. A preview SHALL report the counts without writing anything.

#### Scenario: Two runs do not double up

- **GIVEN** one done task past its window
- **WHEN** housekeeping runs twice
- **THEN** exactly one audit entry records it
