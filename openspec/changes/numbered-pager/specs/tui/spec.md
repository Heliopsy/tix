## ADDED Requirements

### Requirement: The board says which page of a long project it holds

When a project holds more tasks than one read of the board, the status bar SHALL say "page N/M", and
`pgdown` and `pgup` SHALL read the next and previous page by number.

#### Scenario: A project larger than one read

- **GIVEN** a project holding more tasks than one board read
- **WHEN** the reader opens its board and presses pgdown
- **THEN** the status bar says page 2 of the page count and the board holds the next tasks
