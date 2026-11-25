## ADDED Requirements

### Requirement: Mutating routes accept an idempotency key

Every authenticated POST, PUT, PATCH and DELETE route under `/api/v1` SHALL accept an `Idempotency-Key` header of 1 to 200 printable ASCII characters. A key outside that shape, or the header sent twice, SHALL be refused with `400` and code `invalid` before the handler runs. A request without the header SHALL behave as if the feature did not exist.

#### Scenario: A malformed key

- **WHEN** a client creates a task with an `Idempotency-Key` of 201 characters
- **THEN** the server answers 400 with code invalid and creates no task

#### Scenario: No key

- **WHEN** a client sends the same create task request twice without the header
- **THEN** two tasks are created

### Requirement: A repeated request answers the kept response

A key SHALL be scoped to the tenant, the authenticated actor and the key. A request repeating a kept key with the same method, path, query and body SHALL answer the kept status, content type and body without running again, with the header `Idempotent-Replayed: true`. Another actor or tenant using the same key SHALL run independently.

#### Scenario: A replayed create

- **WHEN** a client creates a task with key K and sends the identical request again with key K
- **THEN** the second answer has the same status and body, carries Idempotent-Replayed true, and only one task exists

#### Scenario: Another actor uses the same key

- **WHEN** actor A creates a task with key K and actor B creates a task with key K
- **THEN** both requests run and two tasks exist

### Requirement: A key reused for a different request is refused

A request repeating a kept key with a different method, path, query or body SHALL be refused with `422` and code `invalid`, message "idempotency key reused with a different request", and SHALL NOT run.

#### Scenario: A different body

- **WHEN** a client creates a task titled A with key K and then sends a create titled B with key K
- **THEN** the server answers 422 with code invalid and no task titled B exists

### Requirement: A key whose request is still running answers conflict

A request repeating a key whose first request has not finished SHALL be refused with `409`, code `conflict`, and a `Retry-After` header in seconds, and SHALL NOT run. A first request that stopped before committing anything SHALL release the key once its lease of twice the request timeout lapses.

#### Scenario: A retry during the first request

- **WHEN** a request with key K is still running and the client sends it again
- **THEN** the second answer is 409 with code conflict and a Retry-After header

### Requirement: An applied request never runs twice

A request's write and the record that it ran SHALL commit in one transaction. A repeat of a key whose request committed a write but whose response was not kept SHALL be refused with `409` and code `conflict` without `Retry-After`, and SHALL NOT run.

#### Scenario: The response was lost after the write

- **WHEN** a create task with key K committed and the server stopped before keeping the response, and the client retries after the lease
- **THEN** the server answers 409 with code conflict, no Retry-After, and still only one task exists

### Requirement: Only answers worth repeating are kept

A request that committed a write SHALL keep its response whatever its status. A request that committed nothing SHALL keep a `2xx` response and SHALL release the key for any other status, so a retry with the same key runs again. A response body over 1 MiB SHALL NOT be kept.

#### Scenario: A server error before any write

- **WHEN** a request with key K fails with 500 before committing anything and the client retries with key K
- **THEN** the retry runs

#### Scenario: A validation error

- **WHEN** a create task with key K is refused as invalid and the client retries with key K and the same body
- **THEN** the retry runs and is refused again rather than replayed

### Requirement: The API description names the idempotency header

The generated OpenAPI document SHALL list `Idempotency-Key` as an optional header parameter on every authenticated POST, PUT, PATCH and DELETE operation under `/api/v1`, and on no other operation. `tix mcp` SHALL NOT offer header parameters as tool arguments.

#### Scenario: A mutating operation

- **WHEN** a reader looks up task.create in api/openapi.yaml
- **THEN** it lists an optional Idempotency-Key header parameter
