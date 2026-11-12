## ADDED Requirements

### Requirement: Session renewal is capped by configuration

The server SHALL read `session.max_lifetime` (default 30d) and `session.remember_max_lifetime` (default 90d)
from the file, a `TIX_` variable or a `tix serve` flag, the flag winning only when typed. Zero SHALL turn renewal
off. A set value under a minute, or a negative one, SHALL be refused at start-up naming the key.

#### Scenario: Defaults

- **WHEN** neither key is set
- **THEN** `GET /v1/auth/session-policy` reports `max_lifetime` 30d and `remember_max_lifetime` 90d

#### Scenario: Invalid

- **WHEN** `session.max_lifetime` is `1s`
- **THEN** the server refuses to start, naming `session.max_lifetime`
