## MODIFIED Requirements

### Requirement: Configurable lease TTL

The lease time-to-live SHALL be decided, most specific first, by: the TTL the claim supplies, the task's own
lease, the project's lease default, an authored workflow's `default_lease`, the tenant's `lease_default`, the
deployment's `lease.default`, and finally 30 minutes. The first level that names a figure SHALL apply. The
ceiling SHALL be the lowest of the deployment maximum (24 hours when unset), the tenant's `lease_max` and the
project's `lease_max`. A TTL the claim supplies above the ceiling SHALL be rejected; a figure from any other level
above the ceiling SHALL be clamped to it.

#### Scenario: Global default applied

- **WHEN** a task is claimed and no claim, task, project, workflow or tenant figure is set
- **THEN** the lease expiry reflects the configured deployment default, or 30 minutes when there is none

#### Scenario: Workflow override

- **WHEN** a task is claimed in a project whose authored workflow declares a TTL, and neither the claim, the task nor the project supplies one
- **THEN** the lease expiry reflects the workflow's TTL

#### Scenario: Project default outranks the workflow

- **WHEN** a project sets a lease default and its authored workflow declares a different TTL
- **THEN** a claim on any of its tasks without a TTL of its own is granted the project's figure

#### Scenario: Task lease outranks the project

- **WHEN** a task carries its own lease and its project sets a lease default
- **THEN** a claim on the task without a TTL of its own is granted the task's figure

#### Scenario: Per-claim override

- **WHEN** a claim supplies its own TTL
- **THEN** the lease expiry reflects the supplied TTL regardless of every other level

#### Scenario: A project maximum tightens the tenant's

- **WHEN** a project's lease maximum is lower than the tenant's and the task's own lease is longer than it
- **THEN** the claim is granted the project maximum, and a claim naming a longer TTL is rejected

#### Scenario: Invalid TTL

- **WHEN** a claim supplies a TTL that is zero, negative, or beyond the effective maximum
- **THEN** the claim is rejected with a validation error and no lease is taken

## ADDED Requirements

### Requirement: The effective lease is explained

The service SHALL report, for one task, the lease a claim without a TTL would be granted, the level it comes from,
the effective maximum and the level that sets it. A claim and a renewal SHALL report the same for the lease they
granted.

#### Scenario: Reading where a lease comes from

- **WHEN** a project sets a 2h lease default and the task sets none
- **THEN** the task's lease reads 2h from the project
