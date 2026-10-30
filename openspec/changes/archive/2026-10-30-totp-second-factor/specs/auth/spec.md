## ADDED Requirements

### Requirement: TOTP enrolment

A user SHALL be able to enrol a TOTP authenticator against their global user
record, not against any one tenant actor, so that one enrolment serves every
tenant the user belongs to.

Enrolment SHALL generate a secret from a cryptographically secure source, offer
it as an `otpauth://totp/` URI, and SHALL NOT take effect until the user proves
possession by submitting a correct code. A secret that was generated but never
confirmed SHALL NOT satisfy any requirement to hold a second factor.

#### Scenario: Enrolment requires proof of possession

- **WHEN** a user starts enrolment and abandons it without submitting a code
- **THEN** the account is not enrolled, and a later sign-in asks for no code

#### Scenario: One enrolment covers every tenant

- **WHEN** an enrolled user signs in to a second tenant they are a member of
- **THEN** the same authenticator satisfies the check, and no second enrolment is offered

#### Scenario: Re-enrolment replaces the previous secret

- **WHEN** an enrolled user completes enrolment again
- **THEN** the previous secret stops being accepted and previously issued recovery codes are replaced

### Requirement: TOTP verification parameters

Verification SHALL follow RFC 6238 using HMAC-SHA1, six digits and a thirty
second time step, because authenticator applications widely ignore the
`algorithm` and `digits` parameters of the `otpauth://` URI and assume these
values regardless of what the URI declares.

Verification SHALL accept the immediately preceding, current and following time
steps, to tolerate clock drift in both directions.

#### Scenario: A code from the previous step

- **WHEN** a code generated during the previous time step is submitted
- **THEN** it is accepted

#### Scenario: A code from outside the window

- **WHEN** a code generated more than one time step away is submitted
- **THEN** it is rejected

### Requirement: A code is single use

Each enrolment SHALL record the highest time step it has accepted, and SHALL
reject any code whose time step is less than or equal to that value.

Without this, a code observed over a shoulder, read from a terminal's
scrollback, or repeated by an attacker remains usable for the whole acceptance
window. The window is what makes verification tolerant; this is what keeps a
code single use.

#### Scenario: Replay within the window

- **WHEN** a code is accepted and the same code is submitted again moments later
- **THEN** the second submission is rejected

#### Scenario: The next code still works

- **WHEN** the authenticator produces the code for the following time step
- **THEN** that code is accepted

### Requirement: Secrets are encrypted at rest

A TOTP secret SHALL be stored encrypted under a key held in configuration and
never stored in the database. The user's identifier SHALL be bound into the
encryption so that a ciphertext moved to another user's row fails to open.

When no key is configured, enrolment SHALL be refused. The system SHALL NOT
fall back to storing a secret in plaintext.

#### Scenario: No key configured

- **WHEN** a user attempts to enrol and no TOTP key is configured
- **THEN** enrolment is refused with an error naming the missing configuration, and no secret is written

#### Scenario: A ciphertext moved between rows

- **WHEN** a stored secret is copied into another user's record and that user signs in
- **THEN** verification fails rather than accepting codes from the first user's authenticator

#### Scenario: Diagnostics report the state

- **WHEN** `tix doctor` runs against a deployment holding enrolments
- **THEN** it reports whether a key is configured, and flags enrolments present with no key as unrecoverable

### Requirement: Recovery codes

Enrolment SHALL issue ten single-use recovery codes, each carrying at least 128
bits of entropy from a cryptographically secure source, displayed exactly once
and stored only as hashes.

A recovery code SHALL be accepted anywhere a TOTP code is accepted. A used code
SHALL be marked used rather than deleted, so the audit trail can name which one
was spent.

#### Scenario: A recovery code signs in

- **WHEN** an enrolled user submits a valid unused recovery code instead of a TOTP code
- **THEN** sign-in succeeds and that code is marked used

#### Scenario: A used code is refused

- **WHEN** a recovery code that has already been used is submitted
- **THEN** it is rejected

#### Scenario: Codes are shown once

- **WHEN** a user revisits their security settings after enrolment
- **THEN** the previously issued codes are not displayed again, and the only offer is to regenerate them

### Requirement: Administrative reset

A tenant administrator SHALL be able to clear another user's TOTP enrolment,
which removes the secret and every outstanding recovery code and returns the
user to an unenrolled state.

The reset SHALL be recorded in the audit log naming the administrator who
performed it and the user affected.

#### Scenario: Reset returns the user to unenrolled

- **WHEN** an administrator clears a user's enrolment
- **THEN** that user's next sign-in asks for no code, and is sent through enrolment if the tenant requires a factor

#### Scenario: The reset is attributed

- **WHEN** an administrator clears a user's enrolment
- **THEN** an audit entry records the acting administrator, the affected user, and the time

### Requirement: Per-tenant, per-surface requirement

A tenant SHALL be able to require a second factor independently for the browser,
for the command line, and for SSH. The three settings SHALL be independent, and
SHALL all default to not required.

The requirement SHALL be evaluated when a session is minted, against the tenant
the session is for, and SHALL NOT be re-evaluated on each subsequent request.

#### Scenario: Required on one surface only

- **WHEN** a tenant requires a second factor for the browser and not for SSH
- **THEN** a browser sign-in asks for a code and an SSH session with an enrolled key does not

#### Scenario: A flag changed mid-session

- **WHEN** an administrator enables the requirement while a user holds a live session
- **THEN** that session continues, and the requirement applies the next time a session is minted

### Requirement: An unenrolled user is enrolled, not refused

When a user signs in to a surface whose tenant requires a second factor and that
user holds no enrolment, the system SHALL route them through enrolment and
complete the sign-in on success, rather than refusing them.

Refusing would make enabling the requirement lock out every user including the
administrator enabling it.

#### Scenario: First sign-in after the requirement is enabled

- **WHEN** an unenrolled user signs in to a surface that now requires a factor
- **THEN** they are taken through enrolment, and on confirming a code the sign-in completes

### Requirement: API tokens are exempt

An API token SHALL NOT be subject to any second-factor requirement, on any
surface, because a token carries no interactive channel on which a code could be
requested.

Diagnostics SHALL state this, so that a tenant requiring a factor everywhere is
not misled about what a long-lived token still permits.

#### Scenario: A token authenticates under a full requirement

- **WHEN** a tenant requires a second factor on every surface and an agent presents a valid API token
- **THEN** the request is authenticated and no code is demanded

#### Scenario: Diagnostics name the exemption

- **WHEN** `tix doctor` runs in a tenant requiring a second factor
- **THEN** it reports that API tokens bypass the requirement

### Requirement: Verification is rate limited

After five consecutive failed verifications against one enrolment, further
attempts SHALL be refused for fifteen minutes. A successful verification SHALL
reset the counter.

The limit SHALL be counted against the enrolment rather than the network address
or the session, because an attacker controls those and controls neither the
account nor the clock.

The refusal SHALL be distinguishable from a wrong code, so the user is told to
wait rather than told their authenticator is wrong.

#### Scenario: Lockout after repeated failure

- **WHEN** five consecutive wrong codes are submitted for one enrolment
- **THEN** the next attempt is refused as rate limited, even if the code is correct

#### Scenario: Success clears the counter

- **WHEN** a correct code is accepted after some failures
- **THEN** the failure count returns to zero
