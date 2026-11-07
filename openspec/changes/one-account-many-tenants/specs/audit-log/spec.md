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
