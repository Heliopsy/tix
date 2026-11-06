## ADDED Requirements

### Requirement: The command line manages housekeeping

`tix cleanup show [--project KEY]` SHALL print the tenant policy, the project override and the effective
rules. `tix cleanup set [--project KEY] [--done V] [--cancelled V] [--trash V]` SHALL take `off`, a number of
days such as `30` or `30d`, and with `--project` also `inherit`. `tix cleanup preview [--project KEY]` SHALL
report what would be acted on now, and `tix cleanup run` SHALL run one bounded pass.

#### Scenario: Previewing a project

- **WHEN** `tix cleanup preview --project OPS` runs
- **THEN** it prints how many done, cancelled and trashed tasks would be acted on, and changes nothing
