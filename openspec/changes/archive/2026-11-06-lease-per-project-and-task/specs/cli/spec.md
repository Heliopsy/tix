## ADDED Requirements

### Requirement: The command line sets and shows lease length

`tix task edit REF --lease D` SHALL set the task's lease and `--lease ""` SHALL clear it.
`tix project edit KEY --lease-default D --lease-max D` SHALL set the project's lease bounds and `""` SHALL inherit
the tenant's. `tix task show` SHALL print how long a claim would last and where that comes from, and claim output
SHALL report the length granted and its level.

#### Scenario: Showing where a lease comes from

- **WHEN** a project's lease default is 2h, its task sets none, and `tix task show` runs on the task
- **THEN** it prints `claims last 2h, from the project`
