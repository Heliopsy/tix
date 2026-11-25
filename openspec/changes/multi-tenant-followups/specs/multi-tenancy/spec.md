## ADDED Requirements

### Requirement: Tenant deletion removes the accounts it leaves with no actor

Deleting a tenant SHALL, in the same step, delete every account whose every actor was in that tenant, except a global admin, and SHALL keep an account that is still a member of another tenant. Each deletion SHALL be written to the account log.

#### Scenario: Member of the deleted tenant only

- **WHEN** a global admin deletes tenant `test` and a person is a member of `test` only
- **THEN** their account is deleted and the account log records it

#### Scenario: Member of another tenant too

- **WHEN** a global admin deletes tenant `test` and a person is a member of `test` and HO
- **THEN** their account is kept and sign-in to HO still succeeds

#### Scenario: A global admin with no other membership

- **WHEN** a tenant is deleted and a global admin's only actor was in it
- **THEN** the global admin's account is kept

### Requirement: Diagnostics list accounts with no actor

`tix doctor` SHALL list every account that has no actor in any tenant that is not deleted, with its count, ID and email, and SHALL delete nothing.

#### Scenario: An account left by an earlier tenant deletion

- **WHEN** `tix doctor` runs against a database holding an account whose only tenant was deleted before this change
- **THEN** it reports that account by ID and email and the account still exists afterwards
