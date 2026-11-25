## ADDED Requirements

### Requirement: CalDAV status endpoint

`GET /api/v1/dav` SHALL answer, to an authenticated person, `enabled`, the absolute CalDAV `url` built as every absolute address the API returns is, the exact Basic `username`, `write` (whether the caller's DAV requests may change tasks), `collections` (the choices a password takes) and `reason` when CalDAV is disabled.

#### Scenario: A client asks what to configure

- **WHEN** a signed-in person sends `GET /api/v1/dav` on a server with CalDAV enabled behind a trusted proxy
- **THEN** the answer carries `enabled: true`, a `url` with the proxy's public scheme and host ending `/dav/`, and their sign-in email as `username`

#### Scenario: Read-only before write-back

- **WHEN** a member sends `GET /api/v1/dav` on a server without write-back
- **THEN** the answer carries `write: false`

### Requirement: App password endpoints

The API SHALL expose `GET /api/v1/dav/passwords`, listing the caller's app passwords as metadata, `POST /api/v1/dav/passwords`, taking `name` and optional `collections`, `read_only` and `expires_at` and answering `201` with the password once, its `username` and `url`, and `DELETE /api/v1/dav/passwords/{ID}` answering `204`.

#### Scenario: Creating an app password over HTTP

- **WHEN** a person with a session sends `POST /api/v1/dav/passwords` with `{"name":"iPhone"}`
- **THEN** the answer is `201` with `password`, `username`, `url`, `scopes: ["caldav"]` and `collections: "assigned"`, and a following list carries no password

#### Scenario: A duplicate name is a conflict

- **WHEN** the same person creates a second live app password named `iPhone`
- **THEN** the answer is `409`

### Requirement: A shared host reads the tenant of a DAV credential

On a shared host, a request under `/dav/` SHALL take its tenant from the tenant key in its path, and a request to `/dav/` itself from the tenant of its Basic password, as an API request takes it from `X-Tix-Tenant` or its bearer token. A path key and a password of different tenants SHALL answer `404`.

#### Scenario: Two tenants on one host

- **GIVEN** a shared host serving tenants `acme` and `globex`, and a person with app passwords in both
- **WHEN** their client sends `PROPFIND /dav/` with the `globex` password
- **THEN** the answer names a principal under `/dav/globex/`
