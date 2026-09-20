## ADDED Requirements

### Requirement: Versioned API namespace

The HTTP API SHALL be served under the path prefix `/api/v1`. Breaking changes to request or response shapes SHALL require a new version prefix rather than a change in place.

#### Scenario: Versioned path

- **WHEN** a client requests a task collection at `/api/v1/tasks`
- **THEN** the request is routed to the API and returns a JSON response

#### Scenario: Unversioned path

- **WHEN** a client requests `/api/tasks` without a version segment
- **THEN** the server responds with a not found error in the standard error envelope

### Requirement: Resource routes

The API SHALL expose routes for tasks, projects, workflows, field definitions, dependencies, tags, comments, artifacts, claims, tenants, domains, users, tokens, webhooks, audit entries, export, import, sync, and component bundle export and import. Every operation available through the CLI SHALL have a corresponding HTTP route.

#### Scenario: Coverage

- **WHEN** the set of service operations is compared against the registered HTTP routes
- **THEN** every operation resolves to exactly one route on the real server mux

#### Scenario: Method semantics

- **WHEN** a client sends a request method that a route does not support
- **THEN** the server responds with a method not allowed error in the standard error envelope

### Requirement: Task ref path parameter

A route path parameter identifying a task SHALL accept either the task's stable identifier or its human-readable reference such as `infra-42`. Both forms SHALL resolve to the same task within the resolved tenant.

#### Scenario: Human ref

- **WHEN** a client requests a task at a path containing `infra-42`
- **THEN** the task whose project key is `infra` and whose number is 42 within the resolved tenant is returned

#### Scenario: Stable identifier

- **WHEN** a client requests the same task using its stable identifier
- **THEN** the identical task representation is returned

#### Scenario: Ref from another tenant

- **WHEN** a client presents a ref that exists only in a different tenant
- **THEN** the server responds with a not found error and reveals nothing about the other tenant

### Requirement: Error envelope

Every error response SHALL use a single JSON envelope carrying a machine-readable code, a human-readable message, and optional structured details. The code SHALL map one to one with the service error taxonomy and each code SHALL map to exactly one HTTP status.

#### Scenario: Validation failure

- **WHEN** a request fails field validation
- **THEN** the response carries a validation error code, a message, and details naming each offending field

#### Scenario: Stable mapping

- **WHEN** the same service error is produced by two different routes
- **THEN** both responses carry the identical error code and the identical HTTP status

#### Scenario: No leakage

- **WHEN** an internal failure occurs
- **THEN** the response carries a generic internal error code and message and does not include stack traces, SQL text, or credential material

### Requirement: Keyset pagination

All list endpoints SHALL paginate using a keyset cursor over a stable sort key and identifier, SHALL NOT use OFFSET, and SHALL return an opaque cursor for the next page when more results exist.

#### Scenario: First page

- **WHEN** a client requests a list endpoint without a cursor
- **THEN** the response contains up to the page limit of items and a next cursor when further items exist

#### Scenario: Following the cursor

- **WHEN** a client requests the same endpoint with the returned cursor
- **THEN** the response continues immediately after the last item of the previous page with no duplicates and no gaps among items unchanged since the first page

#### Scenario: Exhausted list

- **WHEN** the final page of a list is returned
- **THEN** no next cursor is present in the response

#### Scenario: Invalid cursor

- **WHEN** a client supplies a malformed or unrecognized cursor
- **THEN** the server responds with a validation error rather than silently returning the first page

### Requirement: Custom field filters on list queries

A list route SHALL accept custom field filters in two forms: a shorthand query parameter per field, spelled `field.<key>=<value>`, and one typed JSON object in a `custom_fields` parameter. A shorthand value SHALL be typed as the JSON scalar it spells, so an unquoted number filters a number and anything that is not JSON stays the string it was typed as. When both forms name the same key, the shorthand SHALL win. A key the store cannot address, a shorthand key given more than once, and a value that is not a string, number, or boolean SHALL each be rejected with a validation error.

#### Scenario: Shorthand parameter

- **WHEN** a client lists tasks with `?field.severity=high`
- **THEN** only tasks whose `severity` field holds the string `high` are returned

#### Scenario: Typed object

- **WHEN** a client lists tasks with `?custom_fields={"points":3}`
- **THEN** only tasks whose `points` field holds the number 3 are returned, and a task holding the string `3` is not

#### Scenario: Shorthand wins over the object

- **WHEN** a client supplies the same key in both forms, as in `?custom_fields={"severity":"low"}&field.severity=high`
- **THEN** the filter applied is the shorthand value `high`

#### Scenario: Malformed key

- **WHEN** a client supplies a key holding anything other than letters, digits, underscores, and dashes, as in `?field.a'b=1`
- **THEN** the server responds with status 400 and an `invalid` error code in the standard envelope

#### Scenario: Unusable value

- **WHEN** a client supplies a value that is neither a string, a number, nor a boolean, as in `?field.severity=[1,2]`
- **THEN** the server responds with status 400 and an `invalid` error code, and no list is returned

### Requirement: Health endpoint

The server SHALL expose a health endpoint that reports whether the process is alive without depending on the database being reachable.

#### Scenario: Process alive

- **WHEN** the health endpoint is requested while the database is unreachable
- **THEN** the endpoint still responds successfully

#### Scenario: No authentication required

- **WHEN** the health endpoint is requested without any credential
- **THEN** the request succeeds and returns no tenant-specific data

### Requirement: Readiness endpoint

The server SHALL expose a readiness endpoint that reports database reachability and whether schema migrations are current. Readiness SHALL report not ready when either check fails.

#### Scenario: Database unreachable

- **WHEN** readiness is requested while the database cannot be reached
- **THEN** the endpoint reports not ready and names the failing check

#### Scenario: Migrations behind

- **WHEN** readiness is requested while the database schema is older than the binary's migration set
- **THEN** the endpoint reports not ready and names the migration check

#### Scenario: Ready

- **WHEN** the database is reachable and migrations are current
- **THEN** the endpoint reports ready

### Requirement: Authentication transport

The API SHALL accept authentication either as a bearer token in the Authorization header or as a session cookie. When both are present, exactly one SHALL be used according to a deterministic order.

#### Scenario: Bearer token

- **WHEN** a request carries a valid API token in the Authorization header
- **THEN** the request is authenticated as that token's actor with that token's scopes

#### Scenario: Session cookie

- **WHEN** a browser request carries a valid session cookie
- **THEN** the request is authenticated as the session's user

#### Scenario: Missing credential when required

- **WHEN** authentication is required and neither credential is present
- **THEN** the server responds with an unauthenticated error in the standard envelope

### Requirement: Tenant resolution precedes authentication

The server SHALL resolve the tenant from the request Host header before authenticating the caller. A credential belonging to a different tenant than the resolved one SHALL be rejected.

#### Scenario: Unknown host

- **WHEN** a request arrives with a Host that maps to no tenant
- **THEN** the server responds with a not found error and does not attempt authentication

#### Scenario: Cross-tenant token

- **WHEN** a token scoped to tenant A is presented on a host that resolves to tenant B
- **THEN** the request is rejected and no data from either tenant is returned

#### Scenario: Matching tenant

- **WHEN** a token scoped to tenant A is presented on a host that resolves to tenant A
- **THEN** the request proceeds and all results are scoped to tenant A

### Requirement: Claim endpoints

The API SHALL expose endpoints to claim a specific task, claim the next unblocked task, renew a lease, and release a claim. When a claim cannot be taken because another worker holds a live lease, the server SHALL respond with a conflict status.

#### Scenario: Contended claim

- **WHEN** a client attempts to claim a task whose lease is held by another worker and has not expired
- **THEN** the server responds with a conflict status and an error code identifying the claim conflict

#### Scenario: Claim next when queue empty

- **WHEN** a client requests the next unblocked task and none is available
- **THEN** the server responds with a distinct empty-queue outcome rather than an error implying failure

#### Scenario: Stale lease token

- **WHEN** a client renews, transitions, or releases using a lease token that is no longer current for the task
- **THEN** the server responds with a conflict status and the task is unchanged

### Requirement: Request size limits

The server SHALL enforce a configurable maximum request body size and SHALL reject a larger body without reading it fully.

#### Scenario: Oversized body

- **WHEN** a client sends a request body exceeding the configured limit
- **THEN** the server responds with a payload too large status in the standard error envelope

#### Scenario: Within limit

- **WHEN** a client sends a body within the configured limit
- **THEN** the request is processed normally

### Requirement: Content type handling

The API SHALL accept JSON request bodies and SHALL respond with JSON. A request carrying a body with an unsupported content type SHALL be rejected.

#### Scenario: Unsupported request type

- **WHEN** a client submits a body with a content type other than JSON to an API route
- **THEN** the server responds with an unsupported media type error

#### Scenario: Malformed JSON

- **WHEN** a client submits a body that is not valid JSON
- **THEN** the server responds with a validation error naming the parse failure and no data is written

#### Scenario: Response type

- **WHEN** any API route responds, including on error
- **THEN** the response declares a JSON content type
