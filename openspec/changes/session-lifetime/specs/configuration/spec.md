## ADDED Requirements

### Requirement: Session lifetime is configurable

The server SHALL read `session.ttl` (default 12h), `session.idle_timeout` (default 0, off) and
`session.remember_ttl` (default 30d, 0 disables Keep me signed in), each from the file, a `TIX_` variable or a
`tix serve` flag, the flag winning only when typed. A set value under a minute, or a negative one, SHALL be
refused at start-up naming the key.

#### Scenario: Default lifetime

- **WHEN** no session key is set and a person signs in
- **THEN** the session expires 12 hours after the sign-in

#### Scenario: Lifetime from the environment

- **WHEN** `TIX_SESSION_TTL=2h` is set and a person signs in
- **THEN** the session expires 2 hours after the sign-in

#### Scenario: Too short

- **WHEN** `session.idle_timeout` is set to 30s
- **THEN** the server refuses to start and names `session.idle_timeout`
