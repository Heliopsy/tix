## ADDED Requirements

### Requirement: The task listing tallies the whole selection on request

`GET /api/v1/tasks` SHALL accept `tally=true` and then answer, beside the page, a `tally` counting every task
the filter selects regardless of the page: `states` as project id, status and count, `blocked` for tasks
waiting on an unfinished dependency and `held` for tasks under a live claim. Without the parameter `tally`
SHALL be absent.

#### Scenario: A paged listing counts every page

- **GIVEN** five tasks in one state
- **WHEN** a caller lists them with `limit=2&tally=true`
- **THEN** the page holds two tasks and `tally.states` counts five

#### Scenario: Not asked

- **WHEN** a caller lists tasks without `tally`
- **THEN** the response carries no `tally`
