## ADDED Requirements

### Requirement: tix audit ls shows and filters the channel

tix audit ls SHALL take repeatable --via and --token flags, SHALL accept via: and token: in --filter, SHALL exit with the usage code for an unknown channel, and its table SHALL carry a VIA column.

#### Scenario: Filtering by channel

- **WHEN** tix audit ls --via ssh runs on a store where nothing was done over SSH
- **THEN** it prints no entries

#### Scenario: The table names the channel

- **WHEN** tix audit ls prints a table
- **THEN** each row's VIA column names its channel and token or job
