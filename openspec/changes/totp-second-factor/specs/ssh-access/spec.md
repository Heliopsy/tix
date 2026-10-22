## ADDED Requirements

### Requirement: A second factor on the SSH surface

When a tenant requires a second factor for SSH, `tix ssh` SHALL request a TOTP
or recovery code after the public key has been accepted and before any tenant
data is drawn.

Ordering matters: the key is proved first, so a key nobody enrolled is refused
without a prompt being drawn and without revealing that an account exists.

A public key is a single factor, not a second one. An SSH session authenticated
by key alone satisfies no second-factor requirement.

#### Scenario: Key accepted, code requested

- **WHEN** an enrolled user connects over SSH to a tenant requiring a factor for SSH
- **THEN** the key is verified, a code is requested, and the board is drawn only after the code is accepted

#### Scenario: An unenrolled key is refused before any prompt

- **WHEN** a key nobody has enrolled connects
- **THEN** the connection is refused without a code prompt being drawn

#### Scenario: SSH not required

- **WHEN** a tenant requires a factor for the browser but not for SSH
- **THEN** an SSH session with an enrolled key reaches the board with no code requested

#### Scenario: A failed code ends the session

- **WHEN** the code prompt is answered incorrectly beyond the permitted attempts
- **THEN** the session is closed and no tenant data has been drawn
