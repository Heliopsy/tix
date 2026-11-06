## ADDED Requirements

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

### Requirement: An existing user is not made an admin of another tenant

An admin email that already belongs to a user SHALL be refused as a conflict naming the address, before
anything is written, because a user belongs to one tenant.

#### Scenario: Naming somebody who already exists

- **WHEN** a tenant is created naming an email already in use
- **THEN** it is refused as a conflict and neither the tenant nor any other named admin exists

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
