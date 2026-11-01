## MODIFIED Requirements

### Requirement: Command surface

The CLI SHALL provide the top-level commands `actor`, `artifact`, `audit`, `bundle`, `claim`, `comment`, `completion`, `config`, `connection`, `ctx`, `demo`, `dep`, `docs`, `doctor`, `domain`, `export`, `field`, `import`, `login`, `logout`, `member`, `project`, `prune`, `retention`, `serve`, `ssh`, `stats`, `status`, `sync`, `tag`, `task`, `tenant`, `theme`, `token`, `tui`, `update`, `user`, `version`, `watch`, `webhook`, and `workflow`. The root help SHALL list exactly these, so a command added to the tree is added here.

#### Scenario: Every group is reachable

- **WHEN** the root help is displayed
- **THEN** each of the listed command groups appears

#### Scenario: Unknown command

- **WHEN** an unrecognized command is invoked
- **THEN** the CLI exits 2 and suggests the closest matching command

#### Scenario: Version output

- **WHEN** `tix version` is run
- **THEN** the version is printed, and `-o json` yields it as structured data along with build metadata

### Requirement: Documented exit codes

The CLI SHALL use exit code 0 for success, 1 for a generic error, 2 for a usage error, 3 for not found or no task available, 4 for conflict or lease expired, 5 for an authentication or authorization failure, 6 for a failed precondition, and 7 when a system tix reads from, such as a sync source, could not be read. These codes SHALL be documented in help output.

#### Scenario: Success

- **WHEN** a command completes normally
- **THEN** it exits 0

#### Scenario: Usage error

- **WHEN** a command is invoked with an unknown flag or a missing required argument
- **THEN** it exits 2 and prints usage to standard error

#### Scenario: Failed precondition

- **WHEN** a command is refused because the data is in a state the operation forbids, such as a transition the workflow does not allow or deleting a task that still has subtasks
- **THEN** it exits 6 rather than the generic error code

#### Scenario: Empty queue

- **WHEN** a claim-next command finds no eligible task
- **THEN** it exits 3

#### Scenario: Lease expired

- **WHEN** a command presents an expired lease token
- **THEN** it exits 4

#### Scenario: Permission denied

- **WHEN** the actor is not permitted to perform the operation
- **THEN** it exits 5

#### Scenario: Upstream unreachable

- **WHEN** a sync run cannot reach the system it imports from
- **THEN** it exits 7
