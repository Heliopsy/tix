## ADDED Requirements

### Requirement: A person can list their own sessions

The service SHALL list the caller's own unexpired sessions in the current tenant, newest first and keyset paginated, each with its identifier, creation and expiry times, sign-in method, whether a second factor passed, the user agent it signed in with, and whether it is the session the caller presented. It SHALL never return a session token or its hash.

#### Scenario: The current session is marked

- **WHEN** a person signed in twice lists their sessions with the first session's cookie
- **THEN** both sessions are listed and only the first is marked current

#### Scenario: Another person's sessions are absent

- **WHEN** a person lists their sessions while another member of the tenant is signed in
- **THEN** none of the other member's sessions are listed

### Requirement: A person can revoke one of their own sessions

The service SHALL end one of the caller's own sessions by identifier, in one transaction with its audit entry and a `session.ended` event, after which the session no longer authenticates. An identifier of another actor's session, or of no session, SHALL be not found and SHALL change nothing.

#### Scenario: Revoking a session of one's own

- **WHEN** a person revokes a session of theirs by identifier
- **THEN** that session's cookie stops authenticating and the audit log records session.revoke

#### Scenario: Revoking someone else's session

- **WHEN** a person revokes the identifier of another member's session
- **THEN** the answer is not found and the other member stays signed in

### Requirement: Session management with a token needs account:read

A session SHALL always list and revoke its own person's sessions. An API token SHALL need the `account:read` scope named among its scopes, which `*` does not carry, and a token pinned to a project SHALL be refused.

#### Scenario: A token holding every scope

- **WHEN** a token with scope `*` and not `account:read` lists sessions
- **THEN** the answer is forbidden

#### Scenario: A token naming account:read

- **WHEN** a token with `account:read` lists sessions
- **THEN** its owner's sessions are listed and none is marked current

### Requirement: A session records the user agent it signed in with

A session minted by a sign-in over HTTP SHALL store the request's user agent, truncated as the audit log truncates it.

#### Scenario: A browser sign-in

- **WHEN** a person signs in from a browser sending a user agent
- **THEN** listing their sessions shows that user agent on the new session
