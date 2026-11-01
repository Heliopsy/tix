## ADDED Requirements

### Requirement: Recurrence columns on tasks

The tasks table SHALL carry `recurrence`, a non-null text column defaulting to the empty string, and
`recurrence_next_id`, a nullable text column naming the successor. The successor column SHALL NOT be a
foreign key, so that deleting the successor never clears the guard against a second spawn. Both SHALL be
added by one forward-only migration applied identically on SQLite and PostgreSQL.

#### Scenario: Upgrade

- **WHEN** a database at the previous schema version is migrated
- **THEN** every existing task carries an empty rule and no successor
