## ADDED Requirements

### Requirement: Snooze is a registered capability

The capability registry SHALL hold `task.snooze` bound to `SnoozeTask` and `task.unsnooze` bound
to `UnsnoozeTask`, each with the `task:write` scope and CLI, HTTP, web and terminal bindings, and
`task.wake` bound to `WakeSnoozed` with CLI and HTTP bindings and the web and terminal recorded as
inapplicable, because waking is a background loop.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** `SnoozeTask`, `UnsnoozeTask` and `WakeSnoozed` each appear exactly once with their bindings
