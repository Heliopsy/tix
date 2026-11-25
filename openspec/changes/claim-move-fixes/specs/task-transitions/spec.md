## ADDED Requirements

### Requirement: A transition dry run judges what the move judges

A transition or route request with `dry_run` SHALL run every check the same request without it runs, against the same task in the same transaction, and SHALL then write nothing. A refused dry run SHALL fail with the same error kind and message as the real request, and an accepted one SHALL answer the task as the move would leave it.

#### Scenario: Dry run to a state the workflow lacks

- **WHEN** `tix task mv REF bogus --dry-run` runs on a task under the default workflow
- **THEN** it exits 2 with the same error the move without `--dry-run` gives, and the task is unchanged

#### Scenario: Dry run to a reachable state

- **WHEN** `tix task mv REF doing --dry-run` runs on an unclaimed task in `todo`
- **THEN** it reports the move as planned and the task is still in `todo` at the same version

#### Scenario: Dry run on a claimed task without the token

- **WHEN** a dry run moves a task another worker holds without its token
- **THEN** it exits 4 with lease_expired, as the move would

### Requirement: A route dry run is judged hop by hop

A route dry run SHALL apply each hop against the state the previous hops would leave, and SHALL report the hops it would apply and where it would stop exactly as the real route does.

#### Scenario: Route through an unknown state

- **WHEN** a route dry run names `doing>bogus` from `todo`
- **THEN** it reports doing as applied and stops at bogus, and the task is still in `todo`

### Requirement: An unknown target state names the workflow's states

A move, route, bulk move or release naming a status that is not a state of the task's workflow SHALL be refused as invalid input whose message lists the workflow's state keys in order and whose details carry them as `states`.

#### Scenario: Move to an unknown state

- **WHEN** a task under the default workflow is moved to `review`
- **THEN** the refusal lists todo, doing, blocked, done and cancelled
