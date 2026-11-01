## ADDED Requirements

### Requirement: A task records its snooze

The tasks table SHALL carry `snoozed_until`, a nullable instant, and `snoozed_by`, a nullable
reference to the actor who set it that is cleared when that actor is deleted, added by migration
0023 with an index on the tenant and wake time. Existing rows SHALL upgrade with both columns
empty, meaning not snoozed.

#### Scenario: Upgrade leaves every task awake

- **GIVEN** a database at migration 0022 holding tasks
- **WHEN** it migrates
- **THEN** every task lists by default as before
