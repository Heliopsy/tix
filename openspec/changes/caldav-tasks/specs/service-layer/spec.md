## ADDED Requirements

### Requirement: App password operations

The service SHALL offer `GetDAVInfo`, `ListDAVPasswords`, `CreateDAVPassword` and `RevokeDAVPassword` on the caller's own app passwords behind the `dav.password` action, which needs task read and is not confinable to a project. The caller SHALL be a person; an agent actor SHALL be refused as forbidden.

#### Scenario: A member creates one without token admin

- **GIVEN** a member, whose role does not hold token admin
- **WHEN** they create an app password
- **THEN** it is created for their own actor with the scope `caldav` alone

#### Scenario: An agent is refused

- **WHEN** an agent's token calls `CreateDAVPassword`
- **THEN** the call fails as forbidden

### Requirement: The caldav scope is minted only as an app password

`CreateToken` SHALL refuse the `caldav` scope as invalid, naming the scopes field, and point to `tix dav password create`.

#### Scenario: token create refuses caldav

- **WHEN** an admin runs `tix token create --scope caldav`
- **THEN** the call fails as invalid and no token is created

### Requirement: App password changes are audited as token changes

Creating and revoking an app password SHALL write the token's row, its `token.create` or `token.revoke` audit entry and its `token.created` or `token.revoked` event in one transaction, with `caldav` in the scopes, and none of them SHALL carry the secret or its hash.

#### Scenario: Revocation is visible and immediate

- **WHEN** a person revokes an app password a client is using
- **THEN** the list shows its `revoked_at`, a `token.revoked` event is written, and the client's next request answers `401`

### Requirement: CalDAV writes go through the ordinary task operations

With write-back, every change a DAV request makes SHALL be made by the existing task update, task creation and multi-hop move, as the owner, with the version the client's ETag names, so its authorization, audit entries, events and conflicts are those of any other surface.

#### Scenario: A change from a phone is an ordinary event

- **WHEN** a client changes a task's title over CalDAV
- **THEN** a `task.updated` event is written whose `via` names the app password, and the task's version increases by one
