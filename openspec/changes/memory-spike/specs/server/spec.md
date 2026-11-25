## ADDED Requirements

### Requirement: Soft memory limit follows the container

When `tix serve` starts in a container with a cgroup memory limit and `GOMEMLIMIT` is not set, the server SHALL set the Go runtime's soft memory limit to 85% of the container's limit and log the chosen limit and its source. When `GOMEMLIMIT` is set, including to `off`, the server SHALL leave the runtime's limit as that variable sets it. Without a container limit the server SHALL set none.

#### Scenario: Container limit

- **WHEN** the server starts with a cgroup memory limit of 512 MiB and no `GOMEMLIMIT`
- **THEN** the runtime's soft memory limit is 85% of 512 MiB and the start log names it

#### Scenario: Operator override

- **WHEN** the server starts with `GOMEMLIMIT=400MiB` in a container limited to 512 MiB
- **THEN** the runtime's soft memory limit is 400 MiB

#### Scenario: No container limit

- **WHEN** the server starts where the cgroup memory limit is `max`
- **THEN** the server sets no soft memory limit
