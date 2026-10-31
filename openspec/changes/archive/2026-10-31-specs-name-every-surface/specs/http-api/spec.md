## ADDED Requirements

### Requirement: Routes beyond the resource collections

Besides the resource routes, the API SHALL serve these routes under `/api/v1`:

- identity and sessions: `GET /whoami`, `POST /auth/login`, `POST /auth/logout`
- the directory: `GET /actors`, `GET /actors/{id}`, and membership at `/members` and
  `/members/{actorID}`
- second factor: `GET`, `POST` and `DELETE /auth/totp`, `POST /auth/totp/confirm`,
  `POST /auth/totp/verify`, `POST /auth/totp/recovery-codes`, and `DELETE /users/{id}/totp` for an
  administrator clearing another user's enrolment
- ssh keys: `/ssh-keys` and `/ssh-keys/{id}`
- task movement and history: `POST /tasks/{ref}/route` for a multi-hop move, `POST
  /tasks/{ref}/restore`, `GET /tasks/{ref}/tree`, `GET /tasks/{ref}/audit` and
  `POST /tasks/{ref}/claim/reclaim`
- retention: `GET` and `PUT /retention`, `POST /prune`
- reports: `GET /stats` and `GET /status`
- live connections: `GET /connections` and `DELETE /connections/{id}`

Each SHALL answer with the standard error envelope on failure and SHALL be held to the same scopes
as the CLI command bound to the same operation.

#### Scenario: Restoring a deleted task over HTTP

- **GIVEN** a soft-deleted task `infra-3`
- **WHEN** a client holding `task:delete` sends `POST /api/v1/tasks/infra-3/restore`
- **THEN** the task is listed again and the response carries it

#### Scenario: A route without its scope

- **WHEN** a client without `tenant:admin` sends `GET /api/v1/connections`
- **THEN** the response is 403 in the standard error envelope

#### Scenario: Every registry route is served

- **WHEN** each HTTP binding in the capability registry is requested on the real server mux
- **THEN** none answers 404 or 405
