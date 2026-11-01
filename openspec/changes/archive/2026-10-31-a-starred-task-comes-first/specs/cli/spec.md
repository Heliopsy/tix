## ADDED Requirements

### Requirement: Starring from the command line

The CLI SHALL offer `tix task star <ref>` and `tix task unstar <ref>`. `tix task ls` SHALL mark a
starred task in its table output, `tix task show` SHALL state whether the task is starred, and the
structured output of both SHALL carry `starred: true` for a starred task. `tix task ls` SHALL
accept `--starred` and `--starred=false`.

#### Scenario: Star then list

- **WHEN** `tix task star OPS-3` is run and then `tix task ls`
- **THEN** OPS-3 is the first row and is marked starred

#### Scenario: Structured output

- **WHEN** `tix task show OPS-3 -o json` is run for a starred task
- **THEN** the document carries `"starred": true`
