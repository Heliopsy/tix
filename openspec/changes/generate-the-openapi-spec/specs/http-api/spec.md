## ADDED Requirements

### Requirement: The API serves its OpenAPI document

The server SHALL answer `GET /api/v1/openapi.json` without a credential and before tenant resolution with the
OpenAPI 3.1 document of the API it runs, as JSON. The document SHALL be generated from the capability registry,
committed as `api/openapi.yaml` and embedded in the binary, and SHALL NOT be edited by hand.

#### Scenario: Reading the document without a credential

- **WHEN** a client sends `GET /api/v1/openapi.json` with no `Authorization` header
- **THEN** the response is `200` with an `application/json` document whose `openapi` field starts with `3.1`

#### Scenario: Every operation names its authority

- **WHEN** an operation of the document needs a credential
- **THEN** its security names the bearer token and the session cookie with the scope the service enforces, and an
  operation served without a credential has an empty security list

### Requirement: The OpenAPI document holds exactly the served routes

The document SHALL describe one operation for every method and path the router registers, and none it does not.
Each operation SHALL carry the registry name as its operation identifier, the first segment of that name as its
tag, its request body, query and path parameters, its success response and the error statuses the error taxonomy
maps to, each answering the standard error envelope.

#### Scenario: A route registered without a registry entry

- **WHEN** a handler is bound to the router with no registry operation or endpoint describing it
- **THEN** the build fails naming the route

#### Scenario: The committed document is stale

- **WHEN** a route, a request or response type, or a doc comment changes and `just openapi` was not run
- **THEN** the build fails naming the first line that differs

### Requirement: Responses match the OpenAPI document

Every request the HTTP API accepts and every response it gives SHALL validate against the document's schema for
that operation and status. A query parameter the operation does not declare SHALL be a contract failure.

#### Scenario: A handler answers a shape the document does not describe

- **WHEN** a handler's response differs from the type its registry entry declares
- **THEN** the API tests fail naming the operation, the status and the field
