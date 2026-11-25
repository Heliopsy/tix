## ADDED Requirements

### Requirement: A person removed from a tenant is not listed as its user

A person whose membership of a tenant is gone SHALL NOT be listed as a user of that tenant, though their
actor stays for history. Their listing in any tenant they still belong to SHALL be unchanged.

#### Scenario: Removed from one of two tenants

- **GIVEN** a person who is a member of tenants A and B
- **WHEN** an admin of B removes them
- **THEN** B's user listing leaves them out and A's user listing still lists them, not marked removed

#### Scenario: Removed by a global admin

- **WHEN** a global admin removes a person from tenant B who also belongs to A
- **THEN** the global admin's listing of B's users leaves them out unless asked to include removed people
