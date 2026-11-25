## ADDED Requirements

### Requirement: The terminal board pages each column on its own

The terminal board SHALL read each workflow state as its own listing, so no page shows a column empty
while its tasks sit on another page. The `!` card marker SHALL be described as waiting on another task.

#### Scenario: A short column on page 2

- **GIVEN** 600 todo tasks and 3 doing tasks
- **WHEN** the reader moves to page 2
- **THEN** todo shows its next tasks and doing shows nothing new, and page 1 showed all 3 doing tasks
