## ADDED Requirements

### Requirement: tix user time

`tix user time` SHALL show each level of the caller's date format and timezone with the values in force,
and `--format` and `--timezone` SHALL set the caller's own, `""` returning a field to the tenant default.
`tix tenant edit` SHALL take `--time-format` and `--timezone` for the tenant default.

#### Scenario: Set and inherit

- **WHEN** a person runs `tix user time --timezone Asia/Tokyo` and then `tix user time --timezone ""`
- **THEN** the second run shows the zone in use coming from the tenant or the deployment

### Requirement: Command line tables follow the account

Table output SHALL write timestamps in the caller's resolved format and zone. A format or zone set on the
machine (a flag, `TIX_OUTPUT_*`, or the configuration file) SHALL win for that field on that machine. A
struct a table has no layout for SHALL write its instants through the same style, never Go's default.

#### Scenario: Unset on the machine, set on the account

- **GIVEN** no `output.time_format` on the machine and an account format of `dotted`
- **WHEN** the person runs `tix task ls`
- **THEN** UPDATED reads like `06.11.2026 14:05`
