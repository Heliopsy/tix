## ADDED Requirements

### Requirement: The command line ticks and clears a checklist item

`tix task check REF N` SHALL tick checklist item N of the task's description and `tix task uncheck REF N`
SHALL clear it, each taking `--version` for optimistic locking and `--dry-run`. A stale version SHALL
exit `4`, and an item number the description does not have, or an N that is not a positive number, SHALL
exit `2`.

#### Scenario: Ticking the third item

- **GIVEN** a task whose body has three open items
- **WHEN** `tix task check default-1 3` runs
- **THEN** only the third item is ticked

### Requirement: Task show states the checklist

In table output `tix task show` SHALL state, on standard error, how many of the description's checklist
items are done, when it has any; JSON and YAML output SHALL carry the `checklist` field.

#### Scenario: The count is stated

- **GIVEN** a task with one of three items ticked
- **WHEN** `tix task show` runs
- **THEN** it prints `checklist: 1/3 done`
