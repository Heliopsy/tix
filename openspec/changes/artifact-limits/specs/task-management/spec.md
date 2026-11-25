## ADDED Requirements

### Requirement: Artifact blob limits

A new artifact blob SHALL be refused when it is larger than the effective size limit or its media type is not
covered by the effective accepted types. The refusal SHALL state the limit or the accepted types. An artifact with
no blob SHALL NOT be judged, and artifacts already stored SHALL NOT be judged again.

#### Scenario: Blob over the limit

- **WHEN** an artifact blob of 2 MiB is put on a tenant whose effective limit is 1 MiB
- **THEN** it is refused as invalid with a message naming 1 MiB, and no artifact is stored

#### Scenario: Type not accepted

- **WHEN** an artifact blob of type text/html is put and text/html is not accepted
- **THEN** it is refused as invalid with a message listing the accepted types

#### Scenario: Limits lowered after the fact

- **WHEN** a tenant lowers its limit below the size of artifacts it already holds
- **THEN** those artifacts are still listed unchanged

### Requirement: Reporting artifact limits

The service SHALL report the effective artifact size limit and accepted types for the current tenant, together
with the server's, to any reader of tasks.

#### Scenario: Limits read over the API

- **WHEN** a client reads `GET /v1/artifact-limits`
- **THEN** it receives `max_bytes`, `allowed_types`, `server_max_bytes` and `server_allowed_types`
