# multi-tenancy Specification

## Purpose

Keeps every tenant's rows in one shared schema and guarantees that no query can read across the tenant boundary.

## Requirements

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

### Requirement: Isolation guards derive their subject from the schema

Every automated assertion that a tenant-owned table is scoped, policied, or isolated SHALL enumerate the
tables from the live database schema and SHALL NOT take its subject from a list maintained in code. Where
a table is exempt from tenant scoping, the exemption SHALL be an explicit allow-list entry naming the
table and recording why it holds no tenant column.

#### Scenario: A table dropped from the scoped list

- **WHEN** a tenant-owned table is removed from the list the isolation policies are generated from
- **THEN** the guard fails and names the table, because it enumerated the schema rather than that list

#### Scenario: A tenant-owned table exempted

- **WHEN** a table carrying a `tenant_id` column is named in the exemption allow-list
- **THEN** the guard fails and reports the reason the exemption gave

#### Scenario: Exemption without a reason

- **WHEN** a table is exempted from tenant scoping with no recorded reason
- **THEN** the guard fails

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

#### Scenario: A read that precedes the tenant

- **WHEN** a lookup must read a tenant-owned table before any tenant is known, such as mapping a hostname or an SSH key fingerprint to its tenant
- **THEN** it is admitted by a narrow `SELECT`-only policy gated on a transaction-local flag that the lookup raises and lowers, and never by exempting the table from scoping

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
- **THEN** the selection is refused with a not-found error and nothing is written

#### Scenario: Selecting a tenant that does not exist yet

- **WHEN** an operator selects a tenant with the check explicitly skipped
- **THEN** the key is written without being verified, so a tenant can be selected before it is created

#### Scenario: Selection belongs with the connection it applies to

- **WHEN** a named context is current and a tenant is selected
- **THEN** the key is stored on that context, because a context pins the database or server the tenant lives in

#### Scenario: Persisting a selection pins nothing else

- **WHEN** a tenant is selected while other settings are being resolved from the environment
- **THEN** only the tenant key is written, and those other settings continue to resolve from the environment afterwards

### Requirement: Tenant selection is reachable from the terminal interface

Switching the tenant a session works in SHALL be reachable from inside the terminal interface, and SHALL NOT require leaving it and relaunching. The interface SHALL offer the tenants the caller is a member of, as listed by whoami, and SHALL still accept a typed tenant key.

#### Scenario: Switching without leaving the interface

- **WHEN** a tenant the caller is a member of is chosen in the terminal interface's tenant view and a credential for it is available
- **THEN** the session works in that tenant from then on, without the process being restarted

#### Scenario: The caller's tenants are listed

- **WHEN** the terminal interface asks which tenant to switch to
- **THEN** it lists the tenants the caller is a member of, with the role held in each, and no other tenant

#### Scenario: No list is offered

- **WHEN** the caller is a member of one tenant only, or is a local caller holding the database
- **THEN** no membership list is offered beyond the current tenant and a typed key is taken instead

#### Scenario: No credential for the chosen tenant

- **WHEN** a remote caller chooses a tenant for which no context holds a credential
- **THEN** the interface stays in its current tenant and says which login command would provide one

#### Scenario: The switch is not written down

- **WHEN** a tenant is switched to from inside the terminal interface
- **THEN** the choice applies to that session only, and the persisted selection is unchanged

### Requirement: A tenant key typed into the terminal interface is checked like the command line's

The typed tenant key SHALL be checked by opening a connection pinned to it and asking that connection who the actor is, which is the same check the command-line selection makes and for the same reason.

#### Scenario: A failed switch changes nothing

- **WHEN** the typed key cannot be reached
- **THEN** the session keeps working in the tenant it was in and the failure names the key

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

### Requirement: A tenant chooses whether deleted tasks are kept

Each tenant SHALL carry a trash setting, on by default, that only a tenant admin MAY change. Changing it
SHALL write one audit entry and one event in the same transaction as the change. Turning it off SHALL NOT
remove any task already in the trash, and turning it back on SHALL make those tasks listable and
restorable again.

#### Scenario: An existing tenant keeps its trash

- **GIVEN** a tenant created before the setting existed
- **WHEN** the reader asks whether the trash is on
- **THEN** it is on

#### Scenario: Turning it off keeps what the trash holds

- **GIVEN** a tenant with one task in the trash
- **WHEN** an admin turns the trash off and back on
- **THEN** the task is in the trash and can be restored

### Requirement: Global admins administer the installation

A user SHALL be a global admin or not, off by default. Only a global admin, or a process holding the
database with no token, SHALL create a tenant, grant or revoke global admin, or reach a tenant other than
its own. A tenant admin SHALL remain confined to its own tenant.

#### Scenario: A tenant admin cannot create a tenant

- **WHEN** a tenant admin who is not a global admin creates a tenant
- **THEN** it is refused as forbidden and no tenant is created

#### Scenario: The first global admin comes from the shell

- **WHEN** `tix --db PATH global-admin grant EMAIL` runs with no token
- **THEN** that user becomes a global admin and the grant is audited

### Requirement: A token carries global admin only when named

The `*` scope SHALL NOT imply `global:admin`. A token SHALL act as a global admin only when it names
`global:admin` and its actor's user is a global admin at the time of the call. Only a global admin SHALL
mint such a token, and only for an actor who is one.

#### Scenario: Escalating through a token

- **WHEN** a tenant admin mints a token naming `global:admin`
- **THEN** it is refused as forbidden

#### Scenario: A revoke reaches a live credential

- **GIVEN** a signed-in global admin
- **WHEN** another global admin revokes them
- **THEN** their next tenant creation is refused

### Requirement: The last global admin stays

Revoking the last enabled global admin SHALL be refused with `precondition_failed`, reason
`last_global_admin`.

#### Scenario: Revoking the only one

- **WHEN** the only global admin is revoked
- **THEN** the revoke is refused and they remain a global admin

### Requirement: A tenant is created with its first admins

Tenant creation SHALL accept first admins, each created as a user with the admin role in the new tenant
only. A password left out SHALL be generated from a secure source, at least 20 characters, and returned
once. If setting up the tenant fails, the tenant SHALL be removed again.

#### Scenario: Two admins with generated passwords

- **WHEN** a global admin creates tenant `test` naming two admin emails without passwords
- **THEN** both can sign in to `test` with the returned passwords, are admins there, and cannot sign in to the caller's tenant

#### Scenario: Setup fails after the row exists

- **WHEN** writing the new tenant's admins fails
- **THEN** no tenant with that key remains and a retry succeeds

### Requirement: Generated passwords are not recorded

A generated password SHALL NOT be stored in plain text, and SHALL NOT appear in any audit entry, event or
log. The audit entry SHALL record the user created and whether its password was generated.

#### Scenario: Searching the trail

- **WHEN** a tenant is created with a generated admin password
- **THEN** no audit entry or event in either tenant contains it

### Requirement: A global admin reaches every tenant

A global admin SHALL list every live tenant, read, rename and delete any tenant, and list, create, change
and delete the users of any tenant. Anybody else SHALL be refused those routes before the tenant is looked
up, so the answer discloses nothing.

#### Scenario: Managing another tenant's users

- **WHEN** a global admin creates a user in tenant `other` and makes it an admin
- **THEN** the user is an admin of `other` only

#### Scenario: A tenant admin names another tenant

- **WHEN** a tenant admin lists the users of tenant `other`, or of a tenant that does not exist
- **THEN** both are refused as forbidden

### Requirement: A person's display name is per tenant

The display name shown for a person as a user of a tenant SHALL be the one on their actor in that
tenant, on every surface. A rename SHALL write that actor, and SHALL also write the account when the
person belongs to that tenant only. A rename in one tenant SHALL NOT change the name shown in another.

#### Scenario: Renaming a two-tenant person

- **GIVEN** a tenant admin who is not a global admin, and a person in that tenant and another
- **WHEN** the admin renames the person on the Users screen
- **THEN** the Users screen, GetUser and ListUsers in that tenant show the new name, and the other tenant still shows the old one

#### Scenario: Renaming a one-tenant person

- **WHEN** a tenant admin renames a person who belongs to that tenant only
- **THEN** the actor and the account both carry the new name, and a membership added later starts with it

### Requirement: A person has one actor per tenant, linked to one account

A user account SHALL be linked to each of its actors through the actor's account reference, with at most one actor per account per tenant. No operation SHALL find an account from an actor, or an actor from an account, by assuming the two share an identifier.

#### Scenario: Second tenant gets a second actor

- **WHEN** a person who is a member of tenant A is added to tenant B
- **THEN** tenant B holds a new actor for that person, linked to the same account, and tenant A's actor is unchanged

#### Scenario: Identifiers that differ still resolve

- **WHEN** a person whose actor identifier differs from their account identifier signs in, passes a second factor, signs in over SSH and mints a token
- **THEN** each step finds the same account and the actor of the tenant it runs in

#### Scenario: No second actor in one tenant

- **WHEN** an account that already has an actor in a tenant is added to that tenant again
- **THEN** it is refused as a conflict and nothing is written

### Requirement: A global admin adds an existing account to a tenant directly

A global admin SHALL be able to add an existing account to any tenant with a role and a handle, without the person's acceptance. The addition SHALL be recorded in the target tenant naming the global admin.

#### Scenario: Two people of HO become admins of test

- **WHEN** a global admin adds villy@ and icek@ to tenant `test` as admins
- **THEN** each holds an admin membership in `test`, keeps their membership in HO, and signs in to either with the same password and second factor

### Requirement: A tenant admin reaches an existing account only by invitation

A tenant admin SHALL NOT attach an existing account to their tenant without the person's acceptance. Creating a user with an address that already has an account SHALL be refused as a conflict. An invite to such an address SHALL grant the membership only when accepted.

#### Scenario: Create with an existing address

- **WHEN** a tenant admin creates a user with an address that has an account in another tenant
- **THEN** it is refused as a conflict and no actor or membership is written

#### Scenario: Invite stays pending until accepted

- **WHEN** a tenant admin invites an address that has an account and the person has not accepted
- **THEN** the person holds no membership in that tenant and cannot be assigned work there

#### Scenario: Invite to an account with no password

- **WHEN** a tenant admin invites an address whose account belongs to another tenant and has no password yet
- **THEN** it is refused as a conflict and nothing is written

### Requirement: A first admin of a new tenant may be an existing account

An admin email of a tenant being created that already has an account SHALL make that account an admin of the new tenant, with its own password and second factor. No password SHALL be generated or returned for it, and supplying one for it SHALL be refused as invalid. A key already in use SHALL be refused as a conflict, and nothing SHALL be created.

#### Scenario: Naming somebody who already exists

- **WHEN** a global admin creates a tenant naming as admin an address that has an account in another tenant
- **THEN** the account becomes an admin of the new tenant, keeps its other memberships, and no initial credential is returned for it

### Requirement: Removing a person from a tenant leaves their other tenants alone

Removing or disabling a person in a tenant SHALL affect only that tenant's actor, membership and credentials, unless the person holds no membership elsewhere, in which case the account is removed or disabled as before. The answer SHALL NOT say which case applied.

#### Scenario: Removed from one of two tenants

- **WHEN** a tenant admin of B removes a person who is also a member of A
- **THEN** the person's sessions, tokens and keys in B stop working, and sign-in to A still succeeds

#### Scenario: Removed from the only tenant

- **WHEN** a tenant admin removes a person who is a member of that tenant only
- **THEN** the account is deleted, as it is today

### Requirement: A session acts in one tenant at a time

A session SHALL belong to one tenant and one actor. Moving to another tenant SHALL mint a new session for the person's actor there, after checking that the membership exists and that neither the account nor that actor is disabled.

#### Scenario: Switch to a tenant the person belongs to

- **WHEN** a person signed in to HO switches to `test`, where they are a member
- **THEN** a session for their `test` actor is minted and later requests act in `test` with their `test` role

#### Scenario: Switch to a tenant the person does not belong to

- **WHEN** a person switches to a tenant where they hold no membership
- **THEN** it is refused as not found, the same answer as for a tenant that does not exist

### Requirement: Only a global admin marks a domain shared

A tenant domain SHALL carry a shared flag, off by default. Only a global admin SHALL set or clear it, for a domain of any tenant. A tenant admin or a token without `global:admin` SHALL be refused as forbidden, whether or not the hostname is mapped, and nothing SHALL change.

#### Scenario: Global admin shares HO's host

- **WHEN** a global admin marks tix.z09.org, a domain of tenant HO, shared
- **THEN** the domain is shared and still belongs to HO

#### Scenario: Tenant admin tries

- **WHEN** an admin of HO who is not a global admin marks HO's own domain shared, by session or by a token carrying `*`
- **THEN** it is refused as forbidden and the domain stays bound to HO alone

### Requirement: Sharing a domain is audited in its tenant

Setting or clearing a domain's shared flag SHALL record an audit entry and an event in the tenant owning the domain, naming the global admin and the value before and after. Setting the value the domain already holds SHALL record nothing.

#### Scenario: Shared and unshared

- **WHEN** a global admin marks a domain of HO shared and later clears it
- **THEN** HO's audit log holds two domain updates, each with the shared value before and after
