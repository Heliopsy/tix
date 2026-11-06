## ADDED Requirements

### Requirement: Quick add on the command line reads dates in the configured order

`tix task add --parse` SHALL read a `due:` date in the order `output.time_format` writes dates, as the
browser and the TUI do. `--due` SHALL keep reading `YYYY-MM-DD` or RFC 3339 only, so a script means the
same date on every machine, and its help SHALL show an example of each.

#### Scenario: A dotted due date in a parsed title

- **GIVEN** `output.time_format: dotted`
- **WHEN** `tix task add --parse 'pay rent due:06.11.2026'` runs
- **THEN** the task falls due on 6 November 2026
