## ADDED Requirements

### Requirement: Default listings leave out snoozed tasks

A task listing that names no snooze term SHALL leave out every task whose wake time is after the
present. Internal readers that walk every task, such as export, copy and project deletion, SHALL
NOT be narrowed by it.

#### Scenario: A snoozed task is not listed

- **GIVEN** OPS-3 is snoozed until tomorrow
- **WHEN** a listing is read with no filter
- **THEN** OPS-3 is not in it

#### Scenario: Export keeps a snoozed task

- **GIVEN** OPS-3 is snoozed until tomorrow
- **WHEN** the tenant is exported
- **THEN** the export holds OPS-3 with its wake time

### Requirement: The filter language selects snoozed tasks

The filter language SHALL accept `is:snoozed` and `snoozed:true` selecting only snoozed tasks;
`-is:snoozed`, `is:awake` and `snoozed:false` selecting only tasks not snoozed; and `snoozed:any`
listing both. The CLI SHALL take `--snoozed`, `--snoozed=false` and `--include-snoozed` for the
same three, and the HTTP API `snoozed=true`, `snoozed=false` and `snoozed=any`. The terminal
interface SHALL answer the same terms against a loaded page.

#### Scenario: Only snoozed tasks

- **GIVEN** OPS-3 is snoozed and OPS-4 is not
- **WHEN** a listing is read with `is:snoozed`
- **THEN** it holds OPS-3 and not OPS-4

#### Scenario: Both

- **GIVEN** OPS-3 is snoozed and OPS-4 is not
- **WHEN** a listing is read with `snoozed:any`
- **THEN** it holds both

#### Scenario: Unknown snoozed value

- **WHEN** a listing is read with `snoozed:maybe`
- **THEN** it fails as a usage error
