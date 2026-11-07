## ADDED Requirements

### Requirement: SSH keys stay enrolled per tenant actor

An SSH key SHALL be enrolled against one tenant's actor and SHALL open only that tenant. A person MAY enrol their own key in every tenant they belong to in one command, which SHALL write one enrolment per tenant, each audited in its tenant.

#### Scenario: Key enrolled in all of a person's tenants

- **WHEN** a member of HO and `test` enrols their key with the all-tenants option
- **THEN** the key is enrolled in both, and connecting with username `test` opens `test`

#### Scenario: Removal from a tenant revokes its keys

- **WHEN** a person is removed from `test`
- **THEN** their keys enrolled in `test` are revoked and their keys in HO keep working
