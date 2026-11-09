## MODIFIED Requirements

### Requirement: Request logging without secrets

The server SHALL log each request with its method, path, status, duration, and the identifier of the tenant the request was resolved to, empty only when no tenant was resolved. Logs SHALL NOT contain password material, session tokens, API token values, webhook signing secrets, or Authorization header contents.

#### Scenario: Request record

- **WHEN** a request completes
- **THEN** a log record exists carrying the method, path, status, duration, and tenant

#### Scenario: Request record names the resolved tenant

- **WHEN** an authenticated request is served in a tenant
- **THEN** its log record carries that tenant's identifier, not an empty value

#### Scenario: Credential redaction

- **WHEN** a request carrying an Authorization header or a session cookie is logged
- **THEN** no part of the credential value appears in the log

#### Scenario: Body not logged

- **WHEN** a request body containing a password field is processed
- **THEN** the password value does not appear in any log record
