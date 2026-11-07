## ADDED Requirements

### Requirement: Actors reference their account

Every actor of kind user SHALL reference its account, unique per tenant. The upgrade SHALL set that reference on every existing user actor to the account with the same identifier, and SHALL change no existing identifier.

#### Scenario: Upgrade of an existing installation

- **WHEN** a database with users created before this change is migrated
- **THEN** each user actor references the account sharing its identifier, and every task, comment, token and audit entry still names the same actor

#### Scenario: Second actor for one account in one tenant

- **WHEN** a second actor referencing the same account is written in a tenant that already holds one
- **THEN** the store refuses it as a conflict

### Requirement: Sessions record how they were minted

Each session SHALL record its sign-in method (`password`, `ssh` or `switch`) and whether a second factor was passed. Sessions existing before the upgrade SHALL read as `password` without a second factor.

#### Scenario: Old session after upgrade

- **WHEN** a session minted before the upgrade is used to switch to a tenant requiring a second factor
- **THEN** a code is asked for
