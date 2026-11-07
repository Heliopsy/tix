## ADDED Requirements

### Requirement: Invites and the password change flag are stored

`users` SHALL carry `require_password_change`, false by default. Invites SHALL live in
`user_invites`, scoped to a tenant, holding the user, email, role, inviter, token hash, creation,
expiry, acceptance and revocation instants, with at most one unaccepted, unrevoked invite per user.
Removing the user's actor or the tenant SHALL remove its invites.

#### Scenario: One pending invite per user

- **WHEN** two unaccepted, unrevoked invites are written for one user
- **THEN** the second write fails
