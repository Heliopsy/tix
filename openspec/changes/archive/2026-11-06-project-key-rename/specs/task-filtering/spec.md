## ADDED Requirements

### Requirement: A project term may name an old key

A `project:` filter term SHALL accept a kept alias of a project and SHALL select that project's
tasks, exactly as its current key does.

#### Scenario: Filtering by an old key

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** tasks are listed with `project:infra`
- **THEN** the tasks of `ops` are returned
