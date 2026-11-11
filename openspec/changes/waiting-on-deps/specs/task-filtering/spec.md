## ADDED Requirements

### Requirement: is:waiting-on selects tasks waiting on a dependency

`is:waiting-on` SHALL select the same tasks as `is:blocked`, which stays accepted.

#### Scenario: The alias

- **WHEN** a filter is parsed from `is:waiting-on`
- **THEN** it selects tasks with an unfinished dependency, as `is:blocked` does
