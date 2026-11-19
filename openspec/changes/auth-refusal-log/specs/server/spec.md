## MODIFIED Requirements

### Requirement: Request logging without secrets

The server SHALL log each request with its method, path, status, duration, resolved tenant, client address and a truncated user agent. The client address SHALL be resolved through the configured trusted proxies. Logs SHALL NOT contain password material, session tokens, API token values, webhook signing secrets, or Authorization header contents.

#### Scenario: Request record

- **WHEN** a request completes
- **THEN** a log record exists carrying the method, path, status, duration, tenant, client address and user agent

#### Scenario: Client address behind a trusted proxy

- **WHEN** a request arrives from a configured trusted proxy with an X-Forwarded-For header
- **THEN** the request record carries the forwarded client address, and from any other peer it carries the peer's address

#### Scenario: Credential redaction

- **WHEN** a request carrying an Authorization header or a session cookie is logged
- **THEN** no part of the credential value appears in the log

#### Scenario: Body not logged

- **WHEN** a request body containing a password field is processed
- **THEN** the password value does not appear in any log record

## ADDED Requirements

### Requirement: Refused authentication is logged with a reason

When the server refuses a credential or a sign-in, it SHALL write one warn record carrying a stable reason code, the client address, the user agent and the request identifier. The response the client receives SHALL NOT change and SHALL NOT carry the reason.

#### Scenario: Expired token

- **WHEN** a request presents an API token past its expiry
- **THEN** the client receives the usual 401 and the refusal record carries reason token_expired with the token's id and name

#### Scenario: No credential

- **WHEN** a request to the API presents no credential
- **THEN** the refusal record carries reason no_credentials

### Requirement: A refused credential is named without its value

A refusal record SHALL name a stored token by its id and name, a well formed token that matched nothing by a reference derived from its stored hash, and a session by the session reference the audit log uses. It SHALL NOT carry the presented value, its stored hash, or a reference for a value that is not of a minted secret's shape.

#### Scenario: Unknown token

- **WHEN** a request presents a well formed token that matches no stored token
- **THEN** the refusal record carries reason unknown_token and a token reference, and neither the token nor its hash appears in the log

#### Scenario: Malformed value

- **WHEN** a request presents a bearer value that is not a token's shape
- **THEN** the refusal record carries reason malformed_token and no reference

### Requirement: Repeated refusals are held back

The server SHALL write at most one refusal record per minute for one client address, reason and token, and the next record it writes for them SHALL carry how many were held back. Refusals SHALL NOT be written to the database.

#### Scenario: A client retrying a bad token

- **WHEN** one client is refused four times in a minute for the same reason
- **THEN** one refusal record is written, and the first refusal after the minute carries suppressed=3

### Requirement: Authorization refusals are logged on a 403

When the policy refuses an authenticated caller and the request is answered 403, the server SHALL write a refusal record with reason scope_denied, project_denied, require_claim or second_factor_required and the caller's token or session. A refusal the request recovered from SHALL NOT be logged.

#### Scenario: Token without the scope

- **WHEN** a token holding only task:read lists projects
- **THEN** the client receives 403 and the refusal record carries reason scope_denied with the token's id and name

### Requirement: SSH refusals carry a reason

Each record the SSH listener writes when it refuses a session SHALL carry a reason code alongside the key fingerprint and source address.

#### Scenario: Unknown key

- **WHEN** a key enrolled nowhere connects to the SSH listener
- **THEN** the refusal record carries reason ssh_unknown_key
