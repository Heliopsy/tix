## ADDED Requirements

### Requirement: Invite and password change endpoints

The API SHALL accept `require_password_change` on `POST /api/v1/users` and `PATCH
/api/v1/users/{id}`, and `new_password` on `POST /api/v1/auth/login`. It SHALL serve `POST|GET
/api/v1/invites`, `DELETE /api/v1/invites/{id}` and `POST /api/v1/invites/{id}/reissue` with
`user:admin`, and `GET|POST /api/v1/auth/invites/{token}` without a credential. An issued invite
SHALL carry its absolute `url`.

#### Scenario: Creating an invite over HTTP

- **WHEN** an administrator sends `POST /api/v1/invites` with `{"email": "ada@example.com"}`
- **THEN** the answer is 201 with a `url` built from the request's own scheme and host
