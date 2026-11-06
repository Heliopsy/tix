## ADDED Requirements

### Requirement: The new-task line reads dates in the configured order

The TUI's new-task prompt SHALL read a `due:` date in the order the configured time format writes dates,
so a reader whose format is `dotted` can type `due:06.11.2026`.

#### Scenario: A dotted date on the new-task line

- **GIVEN** the time format is `dotted`
- **WHEN** a reader types `pay rent due:06.11.2026` at the new-task prompt
- **THEN** the task is created due on 6 November 2026
