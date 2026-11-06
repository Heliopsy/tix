## ADDED Requirements

### Requirement: The board filter reaches the trash

The board's filter bar SHALL accept `is:trashed`, showing only deleted cards, each with its deleted
marker and its restore key. Refusing a restore on a live card SHALL name `is:trashed` as the way to
find the deleted ones.

#### Scenario: Filtering the board to the trash

- **GIVEN** a board with one deleted task and live ones
- **WHEN** the reader filters with `is:trashed`
- **THEN** the service is asked for deleted tasks only and only the deleted card is drawn
