## ADDED Requirements

### Requirement: cli.record_host

The configuration SHALL have a cli.record_host boolean, default false, which adds the host name and operating-system user to the channel of changes a command makes against a database directly.

#### Scenario: Turned on

- **WHEN** TIX_CLI_RECORD_HOST=true and a local command creates a task
- **THEN** the entry records the machine's host name and the OS user
