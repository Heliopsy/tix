## MODIFIED Requirements

### Requirement: A migration runs with its login role's access and the exposure is documented

A migration SHALL run with its login role's access. On PostgreSQL each migration transaction SHALL set
a transaction-local migration flag that a policy on every forced table admits, so a data step reaches
every tenant's rows on a role without BYPASSRLS. Only the migration runner SHALL set that flag, and the
policy SHALL refuse the role runtime transactions enter. The engine package SHALL document this.

#### Scenario: The migration runner's exposure to row-level security is written down

- **WHEN** a reader looks for whether a data migration can be trusted to see the rows it names
- **THEN** the PostgreSQL engine package states how a migration reaches rows under its own policies,
  and which shipped migrations missed them before it did

#### Scenario: A data step on a restricted owner role

- **GIVEN** a PostgreSQL login role that is neither superuser nor BYPASSRLS and owns the tables
- **WHEN** a migration updates rows in two tenants
- **THEN** the rows of both tenants change, as they do on SQLite

#### Scenario: The flag outside a migration

- **WHEN** a runtime transaction scoped to one tenant sets the migration flag
- **THEN** it still sees only that tenant's rows
