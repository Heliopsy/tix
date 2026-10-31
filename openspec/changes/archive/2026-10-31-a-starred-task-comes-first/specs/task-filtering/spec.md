## ADDED Requirements

### Requirement: The filter language selects starred tasks

The filter language SHALL accept `is:starred` and `is:unstarred`, the negated `-is:starred`, and
the boolean spelling `starred:true` / `starred:false`, selecting the tasks the reading actor has
or has not starred. The command line SHALL offer the same selection as `--starred`, and the HTTP
API as the `starred` query parameter. The terminal interface SHALL answer the term in memory the
same way the store does.

The task filter contract's field naming the reading actor SHALL be set by the service from the
caller and SHALL NOT be settable by a caller, so that no reader can list by another actor's
stars.

#### Scenario: Only starred tasks

- **GIVEN** a reader who has starred one of three tasks
- **WHEN** they list with `is:starred`
- **THEN** only the starred task is listed

#### Scenario: Excluding starred tasks

- **WHEN** the same reader lists with `-is:starred`
- **THEN** the two unstarred tasks are listed

#### Scenario: Another reader's stars

- **GIVEN** a task starred by one actor
- **WHEN** a second actor lists with `is:starred`
- **THEN** the task is not listed
