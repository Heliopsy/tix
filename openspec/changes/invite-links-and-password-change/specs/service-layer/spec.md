## ADDED Requirements

### Requirement: Invite operations

The service SHALL offer `CreateInvite`, `ListInvites`, `RevokeInvite` and `ReissueInvite` behind
`user:admin`, and `PreviewInvite` and `AcceptInvite`, which take the token and no actor. Each
change SHALL write its rows, an audit entry and a `user_invite.*` event in one transaction, and no
audit entry, event or log line SHALL carry the token or its hash.

#### Scenario: The token is nowhere in the trail

- **WHEN** an invite is created, re-issued and accepted
- **THEN** no audit entry, event payload or request log line contains any of the tokens

### Requirement: Whom an invite may name

An invite for a new address SHALL create the user, its actor and its membership with no password.
An address of this tenant's user holding no membership SHALL be granted the membership without
touching the password. An address belonging to a member who already has a password, or to a user
of another tenant, SHALL be refused as a conflict.

#### Scenario: An existing account keeps its password

- **GIVEN** a user of this tenant whose membership was removed
- **WHEN** an administrator invites their address as a member
- **THEN** they are a member again and still sign in with their old password

### Requirement: Revoking and re-issuing

Revoking SHALL stop the link at once and keep the account. Re-issuing SHALL revoke the current link
and return a new one with a fresh expiry in one transaction. An accepted invite SHALL be neither.

#### Scenario: A revoked link stops

- **WHEN** an administrator revokes a pending invite
- **THEN** its link shows the not valid message and the account remains without a password
