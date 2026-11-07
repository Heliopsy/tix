## ADDED Requirements

### Requirement: An account can be required to change its password

A user SHALL carry a flag requiring a password change at the next sign-in. A password sign-in on a
flagged account with the right password SHALL answer a `password_change` challenge and mint no
session, on every surface. Sending the same credentials with a new password SHALL complete the
sign-in. A wrong password SHALL answer exactly as it does for an unflagged account.

#### Scenario: A flagged account is challenged

- **GIVEN** a user whose require password change flag is set
- **WHEN** they sign in with the right email and password
- **THEN** the answer is a challenge of kind `password_change` and no session exists

#### Scenario: A wrong password reveals nothing about the flag

- **GIVEN** a flagged user
- **WHEN** somebody signs in with the wrong password
- **THEN** the answer is the same invalid credentials error an unflagged account gives

### Requirement: The replacement password is checked and clears the flag

The new password SHALL meet the password policy and differ from the current one, or be refused as
invalid with the flag left set. Accepting it SHALL store its hash, clear the flag and record a
`user.password_change` audit entry in one transaction. Nothing else SHALL clear the flag except an
administrator turning it off.

#### Scenario: The same password is refused

- **WHEN** a flagged user sends their current password as the new one
- **THEN** the call fails as invalid and the flag stays set

#### Scenario: A changed password signs in

- **WHEN** a flagged user sends a new password of 12 or more characters
- **THEN** a session is minted, the flag is clear, and the next sign-in with the new password needs nothing more

### Requirement: Tokens and keys are not passwords

API tokens and SSH key sign-in SHALL be unaffected by the flag.

#### Scenario: A token keeps working

- **GIVEN** a flagged user holding an API token
- **WHEN** a request presents the token
- **THEN** it is authenticated as before

### Requirement: Invite links

An administrator SHALL be able to invite an email address, which returns a single-use link carrying
a `tix_inv_` secret from a cryptographic random source. Only the secret's SHA-256 hash SHALL be
stored. A link SHALL expire after 7 days unless another expiry between one hour and 30 days is
given.

#### Scenario: An invite returns its link once

- **WHEN** an administrator invites `ada@example.com`
- **THEN** the answer carries a link ending `/invite/tix_inv_...`, and listing the pending invites shows the invite without it

### Requirement: Accepting an invite

Opening a live link SHALL offer to choose a password and a display name, and accepting SHALL set
them, mark the invite used and sign the person in, subject to the tenant's second factor rules. An
account that already has a password SHALL confirm it instead and keep it. A used link SHALL not
work again.

#### Scenario: Accept, sign in, link is dead

- **WHEN** an invited person opens the link, chooses a password and accepts
- **THEN** they are signed in, and opening the link again shows the not valid message

### Requirement: A link that does not work says one thing

An unknown, malformed, expired, used or revoked invite token SHALL answer the same not found with
the same message, on every surface.

#### Scenario: Expired and unknown look alike

- **GIVEN** an invite whose expiry has passed
- **WHEN** its link and a made-up link are opened
- **THEN** both answers have the same status and the same body
