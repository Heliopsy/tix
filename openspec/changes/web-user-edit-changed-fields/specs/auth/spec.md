## ADDED Requirements

### Requirement: An unchanged forced password change is not an account change

An update that sets the forced password change to the value the account already holds SHALL be treated
as leaving it alone, so for a person who belongs to several tenants it is not refused with
`account_in_other_tenants` and the rest of the update applies.

#### Scenario: Role change carrying the current flag

- **GIVEN** a tenant admin who is not a global admin, and a person in that tenant and another who has no forced password change
- **WHEN** the admin changes the person's role with the forced password change set to false
- **THEN** the role is changed and nothing is refused
