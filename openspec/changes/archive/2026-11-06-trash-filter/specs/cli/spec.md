## ADDED Requirements

### Requirement: The command line lists the trash

`tix task ls` SHALL accept `--trashed`, listing only soft-deleted tasks, equivalent to
`--filter is:trashed`.

#### Scenario: Listing the trash

- **GIVEN** a deleted task and a live one
- **WHEN** `tix task ls --trashed` runs
- **THEN** only the deleted task is printed
