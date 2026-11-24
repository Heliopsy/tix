## ADDED Requirements

### Requirement: CalDAV is an access channel

The access channels SHALL include `caldav`: a request under `/dav/` signed in with an app password SHALL record `caldav` with the token's identifier and name, and `--via caldav` SHALL select its entries.

#### Scenario: A change from a reminders app

- **WHEN** a person edits a task's title in Apple Reminders
- **THEN** its `task.update` audit entry records the channel `caldav` and the app password's token name
