## ADDED Requirements

### Requirement: A second step in the browser sign-in

When a tenant requires a second factor for the browser, the sign-in flow SHALL
present a second step after the password is accepted, taking a TOTP or recovery
code, and SHALL establish a session only once the code is accepted.

The second step SHALL work without JavaScript, as every other form in the
browser interface does, and SHALL preserve the `next` destination across both
steps so the reader lands where they were going.

#### Scenario: Password then code

- **WHEN** a user signs in to a tenant requiring a factor for the browser
- **THEN** the password form is followed by a code form, and the session begins only after the code is accepted

#### Scenario: The destination survives both steps

- **WHEN** a user is bounced to sign-in from a deep link and completes both steps
- **THEN** they arrive at the originally requested page

#### Scenario: An incomplete first step grants nothing

- **WHEN** a user passes the password step and abandons the code step
- **THEN** no session exists and no tenant data has been rendered

### Requirement: Enrolment and recovery codes in the browser

The settings screen SHALL offer enrolment, showing the `otpauth://` URI and a
scannable code, and SHALL display the recovery codes exactly once on
confirmation, with a means of copying them.

It SHALL report remaining recovery codes, offer regeneration, and offer removal
of one's own enrolment where the tenant does not require a factor.

#### Scenario: Codes are displayed once

- **WHEN** a user confirms enrolment in the browser
- **THEN** ten recovery codes are shown with a means of copying them, and revisiting settings does not show them again

#### Scenario: Removal is refused where a factor is required

- **WHEN** an enrolled user tries to remove their enrolment in a tenant that requires a factor on the surface they are using
- **THEN** the removal is refused with an explanation

### Requirement: Administrative view of enrolment

A tenant administrator SHALL be able to see which users hold an enrolment and to
clear one, and SHALL NOT be shown any secret or any recovery code.

#### Scenario: The administrator sees state, not secrets

- **WHEN** an administrator views the user list
- **THEN** each user's enrolment state is shown, and no secret or recovery code appears anywhere on the screen
