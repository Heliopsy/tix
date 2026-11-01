## ADDED Requirements

### Requirement: Stars are tenant-owned rows keyed by actor and task

A star SHALL be stored as a row of `task_stars` carrying `tenant_id`, `actor_id`, `task_id` and
`created_at`, unique on the tenant, actor and task. The row SHALL be removed with its task and
with its actor, and SHALL be subject to the same tenant scoping, and on PostgreSQL the same
row-level isolation policy, as every other tenant-owned table.

#### Scenario: One row per actor and task

- **WHEN** one actor stars the same task twice
- **THEN** exactly one star row exists for that actor and task

#### Scenario: Stars are invisible across tenants

- **GIVEN** a star in one tenant
- **WHEN** a listing runs in another tenant
- **THEN** the star is neither read nor able to affect the order
