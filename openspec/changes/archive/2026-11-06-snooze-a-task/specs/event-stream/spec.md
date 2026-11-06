## ADDED Requirements

### Requirement: Snoozing is on the event stream

The event vocabulary SHALL include `task.snoozed` with the task reference, the wake time and the
snoozing actor; `task.unsnoozed` with the reference; and `task.woke` with the reference and the
wake time that passed.

#### Scenario: Subscriber sees a snooze and the wake

- **GIVEN** a subscriber to `task.*`
- **WHEN** OPS-3 is snoozed until 10:00 and a wake pass runs at 10:01
- **THEN** the subscriber receives `task.snoozed` and then `task.woke` for OPS-3
