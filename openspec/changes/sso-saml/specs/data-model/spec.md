## ADDED Requirements

### Requirement: Used SAML assertions are recorded until they expire

The schema SHALL hold `sso_assertions`, tenant-scoped, unique per tenant, provider and assertion id, with the assertion's expiry. Rows past their expiry SHALL be removed as new ones are written, and deleting a provider SHALL delete its rows.

#### Scenario: Duplicate refused

- **WHEN** the same assertion id is recorded twice for one provider
- **THEN** the second write is a conflict
