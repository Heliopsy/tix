## ADDED Requirements

### Requirement: A second factor on the command line

When a tenant requires a second factor for the command line, `tix login` SHALL
prompt for a TOTP or recovery code after the password is accepted, and SHALL
mint a session only once the code is accepted.

The code SHALL NOT be accepted as a command-line flag or as an environment
variable. A live second factor passed as an argument lands in shell history and
in the process table, and the only motivation for wanting it is scripting, which
API tokens already serve.

#### Scenario: Prompted after the password

- **WHEN** a user runs `tix login` against a tenant requiring a factor for the command line
- **THEN** the password is taken first, a code is prompted for, and a session is minted only on success

#### Scenario: No flag carries the code

- **WHEN** a user looks for a way to supply the code non-interactively
- **THEN** no such flag or environment variable exists, and the help text points at API tokens for unattended use

#### Scenario: A non-interactive terminal

- **WHEN** `tix login` runs without a terminal in a tenant requiring a factor
- **THEN** it fails with an error naming API tokens as the unattended path, rather than hanging on a prompt

### Requirement: Managing enrolment from the command line

`tix user totp` SHALL offer enrolment, status and removal of one's own
enrolment, and SHALL allow an administrator to clear another user's.

Enrolment SHALL present the `otpauth://` URI and, where the terminal can render
it, a scannable code, and SHALL display the recovery codes exactly once.

#### Scenario: Enrol from the terminal

- **WHEN** a user runs the enrolment command and submits a correct code
- **THEN** the enrolment is confirmed and ten recovery codes are printed once

#### Scenario: Status reports enrolment without revealing the secret

- **WHEN** a user asks for their TOTP status
- **THEN** it reports whether they are enrolled and how many recovery codes remain, and never prints the secret

#### Scenario: Administrative clearing

- **WHEN** an administrator clears another user's enrolment
- **THEN** the command succeeds, and a user without the administrative scope attempting the same is refused
