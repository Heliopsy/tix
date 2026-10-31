## ADDED Requirements

### Requirement: The filter language selects recurring tasks

The filter expression language SHALL accept `is:recurring`, selecting tasks that carry a recurrence rule,
and its negation `-is:recurring`, selecting tasks that carry none. The CLI, the terminal interface, the
browser and the in-memory matcher SHALL answer it the same way, and the HTTP task listing SHALL accept the
same selection as `recurring=true` or `recurring=false`.

#### Scenario: Recurring only

- **WHEN** tasks are listed with `is:recurring`
- **THEN** only tasks carrying a rule are returned

#### Scenario: Excluding recurring tasks

- **WHEN** tasks are listed with `-is:recurring`
- **THEN** no task carrying a rule is returned
