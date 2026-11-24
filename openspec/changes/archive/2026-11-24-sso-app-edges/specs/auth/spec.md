## MODIFIED Requirements

### Requirement: The second factor of an app sign-in is asked in the browser

An app sign-in SHALL ask the tenant's second factor in the browser when the tenant requires it on the web or on the command line, its second factor policy is required, or the person holds one, and SHALL hand over the code only once it passed. An unenrolled person SHALL enrol inside the sign-in once the requirement or the enforced policy applies, rather than get a session confined to enrolment.

#### Scenario: Cli requirement

- **WHEN** the tenant requires a second factor on the command line only and an app signs in
- **THEN** the browser asks for it before the code is handed over

#### Scenario: Enforced policy

- **WHEN** the tenant's second factor policy is required and past its grace and an unenrolled person signs in from an app
- **THEN** the browser enrols them before any code is handed over, and the exchanged session is not confined to enrolment

### Requirement: A refused app sign-in returns to the app

Once `app_redirect` was validated, a refused sign-in, at the start, the provider's answer or the second factor step, SHALL send the browser to `app_redirect` with `error` set to one of `access_denied`, `not_linked`, `expired`, `provider_error` or `server_error` and the app's `state`, and no code. A wrong or rate limited code SHALL stay on the code form.

#### Scenario: Not linked

- **WHEN** the provider vouches for an email no account here holds
- **THEN** the browser goes to the app's redirect with `error=not_linked`

#### Scenario: Second factor step expired

- **WHEN** the second factor step of an app sign-in is posted after its ticket expired
- **THEN** the browser goes to the app's redirect with `error=expired`

#### Scenario: Wrong code

- **WHEN** a wrong code is posted at the second factor step of an app sign-in
- **THEN** the code form is drawn again and the browser does not go to the app

### Requirement: The app exchanges the code once

`POST /api/v1/auth/sso/exchange` SHALL take `code` and `code_verifier` without a credential and answer as a completed `POST /api/v1/auth/login` does, with a remembered session of method `sso:KEY` and a `user.login` audit entry. An unknown, used or expired code or a verifier whose S256 is not the challenge SHALL be 401 invalid credentials, logged as `sso_code_invalid`. A code SHALL be taken once across replicas, and a take SHALL delete the tenant's expired codes.

#### Scenario: Wrong verifier

- **WHEN** an exchange sends the right code with another verifier
- **THEN** it is 401 and the code cannot be exchanged afterwards

#### Scenario: Race

- **WHEN** several exchanges of one code run at once
- **THEN** exactly one gets a session

#### Scenario: Expired codes pruned

- **WHEN** an exchange runs after other codes in the tenant expired unexchanged
- **THEN** those codes are deleted whether or not the exchange succeeds

## ADDED Requirements

### Requirement: An enrolling app sign-in hands over the code when the app is opened

When an app sign-in completes by enrolling, the browser SHALL show the recovery codes with Open the app as a CSRF protected form carrying a signed handoff valid fifteen minutes, and no code. Posting it SHALL store the code then and answer 302 to `app_redirect` with it. An expired handoff SHALL send the browser to the app with `error=expired`. A handoff SHALL NOT pass as a second factor ticket, nor the reverse.

#### Scenario: Slow reader

- **WHEN** a person enrols during an app sign-in and presses Open the app five minutes later
- **THEN** the browser goes to the app's redirect with a code the app can still exchange

#### Scenario: Handoff expired

- **WHEN** Open the app is pressed sixteen minutes after the enrolment
- **THEN** the browser goes to the app's redirect with `error=expired`

### Requirement: An app sign-in finishes only in the browser

An app's sealed flow, second factor ticket or handoff SHALL be completed only through the browser interface. Presented to the HTTP API, it SHALL be refused as invalid naming `flow`, `ticket` or `app_handoff`, and no code SHALL be stored.

#### Scenario: Flow sent to the API

- **WHEN** `POST /api/v1/auth/sso/complete` carries the flow of an app sign-in
- **THEN** it is 400 naming `flow` and no code is stored
