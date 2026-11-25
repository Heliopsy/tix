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

The server SHALL write at most one refusal record per minute for one client address, reason and token, counting an IPv6 address by its /64, and the next record it writes for them SHALL carry how many were held back. Refusals SHALL NOT be written to the database.

#### Scenario: A client retrying a bad token

- **WHEN** one client is refused four times in a minute for the same reason
- **THEN** one refusal record is written, and the first refusal after the minute carries suppressed=3

#### Scenario: A client cycling through its /64

- **WHEN** one client is refused from several addresses in one IPv6 /64 within a minute
- **THEN** one refusal record is written for them

### Requirement: A held-back count is never lost

When a minute passes, or the server shuts down, with refusals held back that no later refusal record carried, the server SHALL write one held-back record at warn with the client address, reason, token and the count.

#### Scenario: A client that stops

- **WHEN** a client is refused fifty times in a minute and sends nothing more
- **THEN** one refusal record is written, then one held-back record carrying suppressed=49

### Requirement: The refusal log is bounded without evicting clients

The server SHALL count at most 4096 clients at once. A refusal from a client beyond that SHALL share one overflow bucket, written at most once a minute, and SHALL NOT end the count of a client already counted.

#### Scenario: Many addresses at once

- **WHEN** more clients than the bound are refused within one minute
- **THEN** the clients already counted keep their counts, and the rest produce one refusal record and one held-back record marked overflow

### Requirement: Authorization refusals are logged on a 403

When the policy refuses an authenticated caller and the request is answered 403, the server SHALL write a refusal record with reason scope_denied, project_denied, require_claim or second_factor_required and the caller's token or session. A refusal the request recovered from SHALL NOT be logged, as a refusal record or as auth_reason on the request line.

#### Scenario: Token without the scope

- **WHEN** a token holding only task:read lists projects
- **THEN** the client receives 403 and the refusal record carries reason scope_denied with the token's id and name

#### Scenario: A page that went on without an optional part

- **WHEN** a page asks for something its caller's scopes do not cover, goes without it and is answered 200
- **THEN** neither a refusal record nor auth_reason on the request line is written

### Requirement: A server error is not a refusal

A request answered with a status of 500 or above SHALL NOT produce a refusal record or auth_reason, and a second-factor check that failed for any reason but a wrong code or a lock SHALL NOT be noted as a refusal.

#### Scenario: The TOTP key is missing

- **WHEN** a second-factor code arrives and the server cannot open the enrolment's secret
- **THEN** no refusal is noted for it

### Requirement: SSH refusals carry a reason

Each record the SSH listener writes when it refuses a session SHALL carry a reason code alongside the key fingerprint and source address.

#### Scenario: Unknown key

- **WHEN** a key enrolled nowhere connects to the SSH listener
- **THEN** the refusal record carries reason ssh_unknown_key
