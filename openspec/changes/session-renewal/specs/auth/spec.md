## ADDED Requirements

### Requirement: A session in use is renewed

When a request records a session's use, it SHALL move the session's expiry to its window past that use: the
effective `ttl`, or `remember_ttl` for a remembered session. The expiry SHALL never move earlier, and SHALL be
written at most once a minute, with the last-use time.

#### Scenario: Working all day

- **WHEN** a person with the default settings signs in at 08:00 and uses the session every hour
- **THEN** at 21:00 the session is still accepted and expires 12 hours after its last use

#### Scenario: Busy session

- **WHEN** a session makes several requests within one minute
- **THEN** its expiry is written once

### Requirement: Renewal stops at the maximum lifetime

A session SHALL NOT be renewed past its sign-in time plus the server's `session.max_lifetime`, or
`session.remember_max_lifetime` for a remembered session. A zero maximum SHALL turn renewal off. A tenant switch
SHALL keep the original sign-in time.

#### Scenario: Cap reached

- **WHEN** a session in daily use reaches 30 days after its sign-in
- **THEN** its next request after that expiry is unauthenticated

#### Scenario: Switching does not restart the cap

- **WHEN** a person switches tenant 29 days after signing in
- **THEN** the new session expires no later than 30 days after the original sign-in

### Requirement: A removed session is not renewed

A session deleted by sign-out, revocation or removal SHALL NOT be renewed. A session deleted between its lookup
and the write recording its use SHALL be refused as unauthenticated.

#### Scenario: Revoked mid-request

- **WHEN** a session is revoked after its lookup and before its use is recorded
- **THEN** the request is unauthenticated

### Requirement: Renewal is visible to the client

A remembered session's cookie SHALL be sent again, with a `Max-Age` ending at the new expiry, on the response to
a request that renewed it. `GET /whoami` with a session SHALL report `session_expires_at`.

#### Scenario: Remembered cookie follows

- **WHEN** a request renews a remembered session
- **THEN** the response sets the session cookie again with a `Max-Age` to the new expiry
