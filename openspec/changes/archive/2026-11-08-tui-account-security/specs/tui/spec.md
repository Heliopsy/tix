## ADDED Requirements

### Requirement: The terminal interface states the signed-in person's account

The terminal interface SHALL offer an account screen stating who the session is signed in as, the person's second factor, their own API tokens, their own SSH keys and the newest page of their account log. Each section SHALL be read when the screen opens.

#### Scenario: The screen lists what it read

- **WHEN** a reader opens the account screen
- **THEN** it lists the reader's tokens with their scopes and expiry, the reader's keys with their fingerprints, the state of the second factor and the newest account log entries

### Requirement: The account screen survives a refused section

A section the service refuses SHALL say why it is not shown and SHALL NOT cost the reader the rest of the screen.

#### Scenario: A caller who is not a person

- **WHEN** the account log is refused because the caller is not a person
- **THEN** the log section says why
- **AND** the tokens and keys are still listed

### Requirement: The account screen issues a token through the browser's scope picker

The account screen SHALL issue an API token from a name, an expiry chosen from the browser's choices and a set of scopes picked from the browser's scope groups, each with its note. A scope the reader does not hold SHALL be drawn and SHALL NOT be tickable.

#### Scenario: A preset ticks its scopes

- **WHEN** a reader starts a token from a preset
- **THEN** the picker opens with exactly that preset's scopes ticked

#### Scenario: A scope the reader lacks

- **WHEN** a reader tries to tick a scope they do not hold
- **THEN** it stays unticked and the screen says why

### Requirement: An issued token is shown once

The value of an issued token SHALL be drawn on the account screen until the reader dismisses it, and SHALL NOT be written to the status bar.

#### Scenario: The value is dismissed

- **WHEN** a token is issued and the reader presses back
- **THEN** the value is no longer drawn and the screen stays open

### Requirement: The account screen enrols an SSH key

The account screen SHALL enrol a public key in `authorized_keys` form, with an optional label, against the signed-in actor.

#### Scenario: A key is enrolled

- **WHEN** a reader enters a public key and a label
- **THEN** the key is enrolled for the reader and appears in the keys section

### Requirement: The account screen enrols a second factor with a terminal QR code

Starting enrolment SHALL draw the otpauth URI as a QR code when the terminal has colour and room for the whole symbol, and SHALL always draw the secret, grouped in fours, and the URI as text. When the code is not drawn the screen SHALL say why.

#### Scenario: No colour

- **WHEN** enrolment starts on a terminal without colour
- **THEN** no QR code is drawn, the screen says so, and the key and URI are drawn

#### Scenario: The code is confirmed

- **WHEN** the reader enters the code the authenticator shows
- **THEN** the enrolment is confirmed and the recovery codes are drawn once

### Requirement: The account screen replaces recovery codes and removes the second factor behind a confirmation

Replacing the recovery codes and removing the second factor SHALL each be confirmed against a named subject, and a refusal from the service SHALL be shown in the status bar.

#### Scenario: Codes are replaced

- **WHEN** a reader with a second factor agrees to replace the recovery codes
- **THEN** the new codes are drawn once

### Requirement: Revoking a token or a key names its subject

`X` on a token or key SHALL open a confirmation naming it. A revoked one SHALL NOT be revoked again.

#### Scenario: A token is revoked

- **WHEN** a reader selects a token, presses `X` and agrees
- **THEN** that token is revoked and the listing is read again

### Requirement: Every account affordance is gated by the authority it needs

The account screen SHALL offer issuing and revoking tokens and keys only to a reader who may perform them, and its additions form SHALL offer only the subjects the reader may add.

#### Scenario: A reader without token:admin

- **WHEN** a reader who may not issue tokens or enrol keys opens the add form
- **THEN** it offers neither a token nor a key
