## ADDED Requirements

### Requirement: SSH key sign-in routes

`POST /api/v1/auth/ssh/challenge` and `POST /api/v1/auth/ssh/verify` SHALL be reachable without a
credential, after tenant resolution, like `POST /api/v1/auth/login`. The challenge route SHALL answer
`nonce`, `namespace` and `expires_at`, the namespace built from the request's Host. The verify route
SHALL take `public_key`, `signature`, `nonce` and optional `tenant`, SHALL answer a login result shaped
like the password route's, and SHALL set the session cookie when a session is issued.

#### Scenario: A challenge names the host

- **WHEN** a client posts to the challenge route on host `tix.example`
- **THEN** the namespace is `tix-login@tix.example`

#### Scenario: Verifying without a credential

- **WHEN** an anonymous client posts a valid signature to the verify route
- **THEN** it answers 200 with a session
