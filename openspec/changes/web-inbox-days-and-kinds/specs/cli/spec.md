## ADDED Requirements

### Requirement: tix inbox ls --kind

`tix inbox ls` SHALL accept `--kind`, a comma separated list, listing only items of those kinds.

#### Scenario: Mentions only

- **WHEN** a user runs `tix inbox ls --kind mention`
- **THEN** only mention items are printed
