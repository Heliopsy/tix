## ADDED Requirements

### Requirement: The scim:provision scope is named only, exclusive and confined

`scim:provision` SHALL be held only when named: neither `*` nor a role SHALL imply it. A token carrying it SHALL carry no other scope and no project, minting it SHALL need `tenant:admin`, and the token SHALL be refused on every request outside `/scim/v2`.

#### Scenario: A provisioning token cannot read the API

- **WHEN** a token carrying `scim:provision` calls `GET /api/v1/whoami`
- **THEN** it is refused and nothing is read

#### Scenario: Mixing scopes is refused

- **WHEN** an admin runs `tix token create p --scope scim:provision --scope task:read`
- **THEN** the command fails as invalid input

### Requirement: Enabling SCIM mints the tenant's provisioning token

Enabling SCIM SHALL create the tenant's provisioning agent, handle `scim`, when it has none, and SHALL mint a `scim:provision` token for it, shown once, when it holds no live one. Rotating SHALL revoke its live tokens and mint a new one.

#### Scenario: Enable twice

- **WHEN** an admin enables SCIM and enables it again
- **THEN** the first answer carries a token and the second carries none, and one token stays live
