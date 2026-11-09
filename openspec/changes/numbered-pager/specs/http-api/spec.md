## ADDED Requirements

### Requirement: The task listing reports its total

`GET /api/v1/tasks` SHALL answer with `total`, the number of tasks the filter selects, and
`total_capped` true when the count stopped at its cap. It SHALL accept `page=N` as an alternative to
`cursor` and, when given, answer the `page` it served. `next_cursor` SHALL be unchanged.

#### Scenario: Reading the total

- **WHEN** a client lists 12 matching tasks with `limit=5`
- **THEN** the response carries 5 items, `total` 12 and a `next_cursor`

#### Scenario: Asking by page number

- **WHEN** a client asks for `page=3&limit=5` of 12 tasks
- **THEN** the response carries 2 items and `page` 3
