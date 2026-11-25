## ADDED Requirements

### Requirement: The task listing pages by number

`tix task ls` SHALL accept `--page N` to list one page by number, refuse it beside `--cursor` or
`--all`, and say on standard error which page of how many it printed and how many tasks match.

#### Scenario: Listing page 2

- **GIVEN** 12 matching tasks
- **WHEN** `tix task ls --limit 5 --page 2` runs
- **THEN** it prints the sixth to tenth task and reports page 2 of 3 and 12 tasks on standard error
