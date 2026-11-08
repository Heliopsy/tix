## ADDED Requirements

### Requirement: Entries name the person's actor in the tenant they are written in

An audit entry written for a person who belongs to several tenants SHALL name their actor in the tenant the entry belongs to. No entry SHALL name the person's actor in, or the key of, another tenant.

#### Scenario: Same person, two tenants

- **WHEN** a person comments in HO and then in `test`
- **THEN** HO's entry names their HO actor, `test`'s names their `test` actor, and neither names the other tenant

### Requirement: A tenant switch is recorded in the target only

A tenant switch SHALL be recorded in the target tenant as a sign-in with method `switch`, and SHALL NOT be recorded in, or name, the source tenant.

#### Scenario: Switch from HO to test

- **WHEN** a person switches from HO to `test`
- **THEN** `test` records a sign-in by their `test` actor with method `switch`, and HO records nothing about it

### Requirement: Membership grants are attributed

Adding an account to a tenant SHALL be recorded in that tenant naming who added it: the global admin for a direct add, or the inviting admin and then the person on acceptance.

#### Scenario: Global admin adds

- **WHEN** a global admin adds villy@ to `test`
- **THEN** `test` records the addition naming the global admin and marking that a global admin acted

### Requirement: Each account has its own change log

Every change to an account SHALL be recorded in that account's log in the same transaction as the change: credentials, email, second factor, disable, global admin, and memberships added, changed or removed. Each row SHALL name who acted, the tenant involved and the access channel, and SHALL hold no secret.

#### Scenario: Password changed by a global admin

- **WHEN** a global admin acting in HO sets the password of a person who is a member of HO and `test`
- **THEN** the person's account log records a password change by that global admin from HO, without the password or its hash

#### Scenario: Membership added

- **WHEN** a global admin adds the person to `test`
- **THEN** the account log records the membership in `test` with its role and who added it

### Requirement: Only the person and global admins read the account log

The account log SHALL be readable by the account's own person and by global admins, and by nobody else. A tenant admin SHALL be refused, whatever the person's memberships.

#### Scenario: Person reads their log

- **WHEN** a person reads their account log
- **THEN** it lists the changes to their account, newest first, keyset paginated

#### Scenario: Tenant admin asks for it

- **WHEN** a tenant admin who is not a global admin asks for another person's account log
- **THEN** it is refused as not found and nothing is disclosed
