## ADDED Requirements

### Requirement: The caldav scope is named-only and confined to /dav/

The vocabulary SHALL include `caldav`. It SHALL be held only when named, never through `*` or a role. A token carrying it SHALL carry nothing else, no project and no claim requirement, and SHALL be refused with `403` and the refusal reason `caldav_only` on every route outside `/dav/`.

#### Scenario: An app password on the API is refused

- **WHEN** a client sends `GET /api/v1/tasks` with an app password as a bearer token
- **THEN** the answer is `403` and the refusal log records `caldav_only`

#### Scenario: An admin token does not imply caldav

- **WHEN** a request authenticated with a `*` token reaches `/dav/`
- **THEN** it is refused as `dav_password_required`

### Requirement: CalDAV accepts only app passwords over Basic

Under `/dav/` the server SHALL authenticate only HTTP Basic whose password is a live `caldav` token of the resolved tenant. An account password, a session cookie, a bearer header and any other API token SHALL be refused with `401`, a Basic challenge and the refusal reason `dav_password_required`. An account password SHALL never reach the password hasher.

#### Scenario: The account password is refused

- **WHEN** a client sends Basic credentials of a person's email and their account password to `/dav/`
- **THEN** the answer is `401`, the refusal log records `dav_password_required`, and no password hash is computed

#### Scenario: A browser session is not a DAV credential

- **WHEN** a request to `/dav/` carries a valid session cookie and no Authorization header
- **THEN** the answer is `401` with a Basic challenge

### Requirement: The Basic username names the app password's owner

The Basic username SHALL be the owner's sign-in email or their handle in the tenant, compared without regard to case. A valid app password with any other username SHALL be refused with `401` and the refusal reason `dav_user_mismatch`.

#### Scenario: Somebody else's name is refused

- **GIVEN** an app password owned by `ana@example.com`
- **WHEN** a client presents it with the username `raj@example.com`
- **THEN** the answer is `401` and the refusal log records `dav_user_mismatch`

### Requirement: An app password reads as its owner's current role

A DAV request SHALL act as the app password's owner with their current role in the tenant cut to the CalDAV ceiling: task, project and workflow read, and with write-back task write and task transition unless the password is read-only. An owner removed from the tenant or disabled SHALL be refused. Task delete SHALL never be in the ceiling.

#### Scenario: A demotion lands on the next request

- **GIVEN** a member's phone writing over CalDAV
- **WHEN** an admin changes the member's role to viewer and the phone sends `PUT`
- **THEN** the answer is `403` and the task is unchanged

#### Scenario: A removed person's phone stops

- **WHEN** the owner of an app password is removed from the tenant
- **THEN** the next DAV request with it answers `401`

### Requirement: App passwords work for people who sign in only through SSO

A person whose account has no password SHALL be able to create an app password from a session they got through a single sign-on provider, and use it under `/dav/`.

#### Scenario: An SSO-only person syncs their phone

- **GIVEN** a person who signs in only through OIDC
- **WHEN** they create an app password in Settings and add it to a client
- **THEN** the client's `PROPFIND /dav/` answers `207`

### Requirement: A failed DAV sign-in locks nothing

A refused DAV request SHALL be recorded in the refusal log and SHALL NOT lock, slow or otherwise change the owner's password sign-in, single sign-on, second factor, sessions or other tokens.

#### Scenario: A stale phone does not lock its owner out

- **GIVEN** a phone retrying a revoked app password a hundred times
- **WHEN** its owner signs in with their password
- **THEN** the sign-in succeeds
