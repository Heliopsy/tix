## ADDED Requirements

### Requirement: SSO sign-ins and providers are audited

Provider creation, change and deletion SHALL be audited without the secret. A completed SSO sign-in SHALL record `sso.login`, a first link `sso.link`, a just-in-time account `sso.user_created`, and a refusal after the provider vouched for an identity `sso.denied` with the reason.

#### Scenario: Denied recorded

- **WHEN** a sign-in is refused because the email's domain is not allowed
- **THEN** the tenant's audit log holds `sso.denied` naming the provider and the reason
