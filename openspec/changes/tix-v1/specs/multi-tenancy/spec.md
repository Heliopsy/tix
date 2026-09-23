## ADDED Requirements

### Requirement: Shared schema with tenant column

The system SHALL store all tenants in one shared schema rather than one schema or one database per
tenant. Every tenant-owned table SHALL carry a `tenant_id` column, and `tenant_id` SHALL be the
leading column of every composite index on those tables.

#### Scenario: Index leads with tenant

- **WHEN** the schema is inspected after migration
- **THEN** every composite index on a tenant-owned table begins with `tenant_id`

#### Scenario: One migration path for all tenants

- **WHEN** a schema migration is applied to a deployment holding many tenants
- **THEN** it runs once and applies to every tenant

#### Scenario: Listing stays selective at scale

- **WHEN** a tenant lists its tasks in a database holding a million tasks across many tenants
- **THEN** the query is served through a tenant-leading index and does not scan other tenants' rows

### Requirement: Scoped query builder

Every database statement against a tenant-owned table SHALL be constructed through a query builder
that requires a tenant scope. The builder SHALL NOT expose any API capable of producing a statement
against a tenant-owned table without a `tenant_id` predicate.

#### Scenario: Builder requires a scope

- **WHEN** code attempts to build a query against a tenant-owned table without supplying a tenant scope
- **THEN** the code does not compile or the builder returns an error, and no statement is produced

#### Scenario: Predicate always present

- **WHEN** any select, update, or delete is generated for a tenant-owned table
- **THEN** the generated SQL contains a `tenant_id` predicate bound to the caller's tenant

#### Scenario: Inserts carry the scope

- **WHEN** an insert into a tenant-owned table is generated
- **THEN** the `tenant_id` value written is taken from the tenant scope, not from caller-supplied fields

### Requirement: Raw query construction is forbidden

The build SHALL fail when raw database query or execute calls appear outside the scoped query
builder package. This check SHALL run as part of continuous integration and SHALL NOT be advisory.

#### Scenario: Raw call added elsewhere

- **WHEN** a raw database query or execute call is added in a package other than the query builder
- **THEN** the lint step fails and names the file and line

#### Scenario: Builder package is exempt

- **WHEN** the query builder package itself issues raw calls
- **THEN** the check permits them, because that package is the single controlled boundary

### Requirement: Generated statement assertion

An automated test SHALL enumerate the statements the system can generate against tenant-owned
tables and SHALL assert that each one carries a `tenant_id` predicate or, for inserts, a
`tenant_id` value. The test SHALL fail if any statement lacks it.

#### Scenario: All statements pass

- **WHEN** the assertion test runs against the current statement set
- **THEN** every generated statement is reported as tenant-scoped

#### Scenario: Unscoped statement introduced

- **WHEN** a change introduces a statement over a tenant-owned table with no tenant predicate
- **THEN** the test fails and identifies the offending statement

### Requirement: Database-enforced isolation on PostgreSQL

On PostgreSQL the system SHALL enable row-level security on every tenant-owned table and SHALL set
a per-transaction `tix.tenant_id` setting that the policies read. The database SHALL reject or
filter out rows belonging to any other tenant even when the application layer is defective.

#### Scenario: Setting established per transaction

- **WHEN** a transaction begins on PostgreSQL for a request
- **THEN** `tix.tenant_id` is set to the resolved tenant for the duration of that transaction

#### Scenario: Policy blocks a leaking query

- **WHEN** a deliberately unscoped query is executed inside a tenant transaction during testing
- **THEN** no rows belonging to another tenant are returned

#### Scenario: Cross-tenant insert refused

- **WHEN** an insert supplies a `tenant_id` other than the transaction's tenant
- **THEN** the database rejects the write

#### Scenario: SQLite lacks this layer

- **WHEN** a deployment runs on SQLite
- **THEN** the diagnostic command states that row-level security is unavailable and recommends PostgreSQL for shared deployments

### Requirement: Tenants

The system SHALL support multiple tenants, each with a stable identifier, a human-readable name, and
a slug unique across the deployment. A tenant SHALL own its projects, workflows, tags, field
definitions, and tasks.

#### Scenario: Tenant created

- **WHEN** a tenant is created with a name and slug
- **THEN** it is assigned an identifier and its slug is reserved across the deployment

#### Scenario: Duplicate slug

- **WHEN** a tenant is created with a slug already in use
- **THEN** the creation is rejected with a conflict error

#### Scenario: Owned resources follow the tenant

- **WHEN** a project is created
- **THEN** it belongs to exactly one tenant and cannot be listed from another

### Requirement: Memberships and per-tenant roles

Users SHALL exist as a single global row. Access to a tenant SHALL be granted through a
`tenant_members` row that assigns that user a role within that tenant. A user SHALL be able to hold
different roles in different tenants, and absence of a membership row SHALL mean no access.

#### Scenario: One identity, several tenants

- **WHEN** a user is a member of two tenants
- **THEN** one set of credentials authenticates them and each tenant applies its own role

#### Scenario: Different role per tenant

- **WHEN** a user is an administrator in one tenant and a read-only member in another
- **THEN** administrative operations succeed in the first and are refused in the second

#### Scenario: No membership

- **WHEN** an authenticated user with no membership in a tenant requests that tenant's data
- **THEN** access is refused and no tenant data is disclosed

#### Scenario: Membership revoked

- **WHEN** a membership is removed
- **THEN** subsequent requests for that tenant are refused even if the user's session remains valid

### Requirement: API tokens are tenant-scoped

Every API token SHALL belong to exactly one tenant and SHALL grant access only to that tenant's
data. The system SHALL NOT issue a token valid across multiple tenants.

#### Scenario: Token limited to its tenant

- **WHEN** a token issued for tenant A is used to request tenant A's tasks
- **THEN** the request succeeds

#### Scenario: Token used against another tenant

- **WHEN** a token issued for tenant A is used on a request resolved to tenant B
- **THEN** the request is rejected with an authentication or authorization error and no tenant B data is returned

#### Scenario: Token creation requires a tenant

- **WHEN** a token is created
- **THEN** a tenant is recorded on it and there is no option to create a tenant-independent token

### Requirement: Implicit default tenant

On first use the system SHALL create an implicit tenant with the slug `default` and SHALL resolve
every request to it when no tenant is otherwise determined. Single-user local use SHALL NOT require
naming, creating, or selecting a tenant.

#### Scenario: Fresh local install

- **WHEN** a user runs the CLI against a new local database with no configuration
- **THEN** the default tenant is created transparently and no tenant argument is required

#### Scenario: Tenancy stays invisible

- **WHEN** a single-user local install creates projects and tasks
- **THEN** no output requires the user to understand tenants

#### Scenario: Default tenant is a normal tenant

- **WHEN** a deployment later adds a second tenant
- **THEN** the default tenant continues to work unchanged and is subject to the same isolation rules

### Requirement: Tenant selection is reachable from the command line

Selecting the tenant that later commands work in SHALL be an operation, not an instruction to edit a configuration file. The selection SHALL be persisted, SHALL be overridable for a single command by the global tenant flag, and SHALL be reported by the command that shows the effective configuration.

#### Scenario: Select a tenant

- **WHEN** an operator selects a tenant by key
- **THEN** later commands run against that tenant without naming it, and the command that shows the effective configuration reports it

#### Scenario: A single command still overrides

- **WHEN** a command is run with the global tenant flag after a tenant has been selected
- **THEN** that one command uses the flag's tenant and the persisted selection is unchanged

#### Scenario: Selection is verified against the target

- **WHEN** an operator selects a tenant that cannot be reached with the current configuration
- **THEN** the selection is refused with a not-found error and nothing is written, because an actor is bound to one tenant and so cannot list another to validate the key against

#### Scenario: Selecting a tenant that does not exist yet

- **WHEN** an operator selects a tenant with the check explicitly skipped
- **THEN** the key is written without being verified, so a tenant can be selected before it is created

#### Scenario: Selection belongs with the connection it applies to

- **WHEN** a named context is current and a tenant is selected
- **THEN** the key is stored on that context, because a context pins the database or server the tenant lives in

#### Scenario: Persisting a selection pins nothing else

- **WHEN** a tenant is selected while other settings are being resolved from the environment
- **THEN** only the tenant key is written, and those other settings continue to resolve from the environment afterwards

### Requirement: Tenant resolved once per request

The tenant for a request SHALL be resolved once, before any service method executes, and SHALL be
carried in the request context. Service methods SHALL take the tenant from that context and SHALL
NOT accept a tenant supplied in a request body or query parameter.

#### Scenario: Body-supplied tenant ignored

- **WHEN** a request body contains a tenant identifier different from the resolved tenant
- **THEN** the resolved tenant is used and the supplied value has no effect

#### Scenario: Tenant unchanged mid-request

- **WHEN** a single request performs several service calls
- **THEN** all of them operate under the same tenant

#### Scenario: Unresolved tenant

- **WHEN** a request arrives and no tenant can be resolved
- **THEN** the request is refused before any tenant-owned data is read

### Requirement: Cross-tenant leak test suite

The test suite SHALL seed at least two tenants with deliberately similar data, including matching
project keys, tag names, and task titles, and SHALL assert that every service method, every HTTP
API route, and every web handler returns no data from the other tenant.

#### Scenario: Every service method covered

- **WHEN** the leak suite runs
- **THEN** each exported service method is exercised under tenant A and asserted to return nothing belonging to tenant B

#### Scenario: Every route covered

- **WHEN** the leak suite exercises each HTTP route and web handler under tenant A's credentials
- **THEN** no response contains a tenant B identifier, title, or reference

#### Scenario: Identifier guessing

- **WHEN** a caller in tenant A requests a tenant B entity by its exact identifier
- **THEN** the response is a not-found result that does not reveal the entity's existence

#### Scenario: New surface without coverage

- **WHEN** a new service method or route is added without a leak-suite case
- **THEN** the suite fails and names the uncovered surface

### Requirement: Tenant deletion

Deleting a tenant SHALL soft-delete the tenant and make its data inaccessible through every access
path. Purging a tenant's data permanently SHALL be an explicit, separately confirmed operation.

#### Scenario: Soft-deleted tenant is inaccessible

- **WHEN** a tenant is deleted and one of its members attempts to use it
- **THEN** the request is refused and no tenant data is returned

#### Scenario: Domains released

- **WHEN** a tenant is deleted
- **THEN** its domains no longer resolve to it and may be reassigned

#### Scenario: Purge is explicit

- **WHEN** an operator purges a deleted tenant
- **THEN** the operation requires explicit confirmation and reports how many rows were removed
