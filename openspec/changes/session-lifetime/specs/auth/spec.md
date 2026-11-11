## ADDED Requirements

### Requirement: A session unused past the idle timeout is refused

A session SHALL record when it last authenticated a request, writing that time at most once a minute. When the
effective idle timeout is set, a session whose last use is older than it SHALL be refused as unauthenticated.
The idle timeout SHALL be read on every request, so a new value applies to live sessions at once, remembered
sessions included.

#### Scenario: Idle session

- **WHEN** the tenant's idle timeout is one hour and a session was last used 61 minutes ago
- **THEN** the next request with it is unauthenticated

#### Scenario: Busy session

- **WHEN** a session makes several requests within one minute
- **THEN** its last use is written once

### Requirement: Keep me signed in mints a longer session

A sign-in that asks to be kept signed in SHALL last the effective remember lifetime when that is longer than
the ordinary lifetime, and SHALL be marked remembered. Otherwise it SHALL be an ordinary session. The choice
SHALL be honoured on the step that completes the sign-in, after a second factor or a password change.

#### Scenario: Remembered

- **WHEN** a person signs in with remember on a server with the default settings
- **THEN** the session expires 30 days later and is marked remembered

#### Scenario: Not offered

- **WHEN** the tenant turned Keep me signed in off and a person signs in with remember
- **THEN** the session expires after the ordinary lifetime and is not marked remembered

### Requirement: Session cookie lifetime matches the session

A remembered session's cookie SHALL carry `Expires` and a `Max-Age` equal to the session's lifetime. Every
other session's cookie SHALL carry neither, so it ends with the browser.

#### Scenario: Ordinary sign-in

- **WHEN** a browser signs in without Keep me signed in
- **THEN** the session cookie has no `Max-Age` and no `Expires`

### Requirement: New lifetimes apply to new sign-ins

A change to the lifetime or remember lifetime SHALL leave the expiry of every existing session unchanged.

#### Scenario: Shortened after sign-in

- **WHEN** a tenant shortens its lifetime to one hour after a person signed in for 12 hours
- **THEN** that session still expires 12 hours after its sign-in and the next sign-in lasts one hour

### Requirement: The session policy is readable without a sign-in

`SessionPolicy` SHALL report the effective lifetime, idle timeout and remember lifetime for the request's
tenant beside the server's, and SHALL need no credential.

#### Scenario: Signed out

- **WHEN** an anonymous client asks for the session policy
- **THEN** it gets the tenant's values in force
