## ADDED Requirements

### Requirement: SSH keys stay enrolled per tenant actor

An SSH key SHALL be enrolled against one tenant's actor and SHALL open only that tenant. A person MAY enrol their own key in every tenant they belong to in one command, which SHALL write one enrolment per tenant, each audited in its tenant.

#### Scenario: Key enrolled in all of a person's tenants

- **WHEN** a member of HO and `test` enrols their key with the all-tenants option
- **THEN** the key is enrolled in both, and connecting with username `test` opens `test`

#### Scenario: A token without account:read

- **WHEN** a token carrying `*` but not `account:read` asks to enrol a key in all of its owner's tenants
- **THEN** it is refused as forbidden and nothing is written in any tenant

#### Scenario: A tenant where the person may not enrol keys

- **WHEN** a person enrols their key in all tenants and their role in one of them may not enrol keys
- **THEN** that tenant is passed over and the others are enrolled

#### Scenario: Removal from a tenant revokes its keys

- **WHEN** a person is removed from `test`
- **THEN** their keys enrolled in `test` are revoked and their keys in HO keep working

### Requirement: The terminal interface over SSH names the tenant username

A terminal interface session reached over SSH SHALL list the person's memberships in its tenant view, and SHALL answer the choice of another tenant with the SSH command naming that tenant as the username rather than switching.

#### Scenario: Choosing another tenant over SSH

- **WHEN** a member of HO and `test` connected as `ho@host` chooses `test` in the tenant view
- **THEN** the session stays in HO and names `ssh test@HOST`
