## ADDED Requirements

### Requirement: View arrangement operations

The service SHALL offer `ListGroupPrefs`, `SetGroupPref`, `OrderGroups` and `ResetGroupPrefs`,
each acting only on the caller's own preferences and requiring the `task:read` scope. A view SHALL
be `board`, `lanes` or `project:` followed by a project reference. A `lanes` group SHALL be a
project of the tenant; a `project:` group SHALL be a state of that project's workflow; a `board`
group SHALL be a state of some workflow of the tenant. `OrderGroups` SHALL write positions 1..n for
the groups named, in that order, and SHALL refuse a duplicate. Every operation SHALL be registered
with CLI, HTTP and web bindings.

#### Scenario: A state the workflow lacks is refused

- **WHEN** a caller hides group `shipped` on `project:infra` whose workflow has no such state
- **THEN** the call fails as invalid and nothing is written

#### Scenario: A project-pinned token cannot arrange

- **WHEN** a token pinned to one project calls `SetGroupPref`
- **THEN** the call is refused as forbidden
