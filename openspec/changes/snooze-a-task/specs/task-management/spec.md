## ADDED Requirements

### Requirement: A task can be snoozed until a moment in the future

The service SHALL let an actor with task write snooze a live task until an instant after the
present. Snoozing SHALL record the instant and the snoozing actor on the task, write a
`task.snooze` audit entry naming that actor with the previous and new wake times, and emit
`task.snoozed`, all in one transaction. Snoozing an already snoozed task SHALL replace its wake
time. A wake time that is not after the present SHALL be refused as invalid, and a deleted task
SHALL be not found. A snooze SHALL be visible to every reader of the task, not only to the actor
who set it.

#### Scenario: Snoozing hides the task from everyone

- **GIVEN** alice and bob both read project OPS
- **WHEN** alice snoozes OPS-3 until tomorrow
- **THEN** OPS-3 is absent from bob's default listing and the audit entry for it names alice

#### Scenario: A wake time in the past is refused

- **WHEN** an actor snoozes OPS-3 until an hour ago
- **THEN** the call fails as invalid and nothing is written

### Requirement: A snoozed task can be unsnoozed

The service SHALL let an actor with task write unsnooze a task, clearing its wake time and
snoozing actor, writing `task.unsnooze` and emitting `task.unsnoozed` in one transaction.
Unsnoozing a task that is not snoozed SHALL change nothing and write nothing.

#### Scenario: Unsnoozing brings the task back at once

- **GIVEN** OPS-3 is snoozed until next week
- **WHEN** an actor unsnoozes it
- **THEN** OPS-3 is in the default listing again

### Requirement: A snoozed task wakes by itself

A task SHALL count as snoozed only while its wake time is after the present, judged at the moment
of each read, so it returns to every listing when the time passes without any process running. A
wake pass SHALL clear every wake time that has passed, recording `task.wake` and emitting
`task.woke` for each in one transaction, and SHALL skip a task whose wake time changed since the
pass read it. A running server SHALL run a wake pass on the same interval as its lease sweep.

#### Scenario: Time passing is enough

- **GIVEN** OPS-3 is snoozed until 10:00 and no server is running
- **WHEN** a listing is read at 10:01
- **THEN** OPS-3 is in it

#### Scenario: The wake pass announces the task

- **GIVEN** OPS-3 was snoozed until 10:00
- **WHEN** a wake pass runs at 10:01
- **THEN** OPS-3 no longer carries a wake time and `task.woke` was emitted for it

### Requirement: Reading one task shows its snooze

Reading a task by reference SHALL return it whether or not it is snoozed, with its wake time and
the actor who snoozed it while the snooze is in force.

#### Scenario: Show a snoozed task

- **GIVEN** OPS-3 is snoozed
- **WHEN** an actor shows OPS-3
- **THEN** the task is returned with `snoozed_until` and `snoozed_by_actor_id`
