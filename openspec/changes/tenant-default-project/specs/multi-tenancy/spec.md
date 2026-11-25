## MODIFIED Requirements

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

#### Scenario: A new tenant opens on its Default project

- **WHEN** a tenant is created
- **THEN** it has a project keyed `default` named Default, with the icon and colour the starter Default
  list has on a fresh installation, and creating it again leaves that project alone
