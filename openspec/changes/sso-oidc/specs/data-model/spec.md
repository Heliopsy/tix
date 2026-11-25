## ADDED Requirements

### Requirement: External identities are stored per tenant and provider

The schema SHALL hold `sso_providers` and `user_identities`, both tenant-scoped, with an identity unique per tenant, provider and subject, and per tenant, provider and user. Deleting a provider SHALL delete its identities. `users.sso_subject` and `users.sso_provider` SHALL be dropped.

#### Scenario: Provider removed

- **WHEN** a provider is deleted
- **THEN** its linked identities are gone and the accounts remain
