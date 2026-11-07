## ADDED Requirements

### Requirement: The terminal reads the account's date settings

The terminal interface, locally and over SSH, SHALL draw every timestamp, the activity clock included, in
the person's resolved style. Its settings screen SHALL show the account's value and its source for a
date field the machine leaves unset, and say that a value picked there applies to this machine only.

#### Scenario: Account value shown

- **GIVEN** a tenant default of `long` and nothing on the machine
- **WHEN** the person opens the settings screen
- **THEN** the time format row reads `long` with a note that it comes from the tenant default
