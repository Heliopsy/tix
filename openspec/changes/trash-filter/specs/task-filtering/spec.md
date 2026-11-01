## ADDED Requirements

### Requirement: The filter language selects the trash

The filter language SHALL accept `is:trashed`, selecting only soft-deleted tasks, and its negation
`-is:trashed`, selecting only live tasks. When either form is present it SHALL decide whether deleted
tasks are listed, whatever `is:deleted` says, so `is:deleted -is:trashed` SHALL list live tasks only.
Both storage engines SHALL answer it in the listing query with keyset pagination unchanged, and the
in-memory matcher SHALL answer it the same way.

#### Scenario: Only deleted tasks

- **GIVEN** three tasks, one of them soft-deleted
- **WHEN** tasks are listed with `is:trashed`
- **THEN** only the deleted task is returned

#### Scenario: The trash pages like any listing

- **GIVEN** three soft-deleted tasks and two live ones
- **WHEN** tasks are listed with `is:trashed` one per page, following the cursor
- **THEN** the three deleted tasks are returned, each once, and no live task is

#### Scenario: Live only, whatever is:deleted says

- **WHEN** tasks are listed with `is:deleted -is:trashed`
- **THEN** no deleted task is returned
