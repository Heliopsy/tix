## ADDED Requirements

### Requirement: A native app starts a sign-in through a provider in the browser

A sign-in through any enabled provider SHALL accept `app_redirect`, `code_challenge`, `code_challenge_method` and `app_state` at `/login/sso/KEY`. `app_redirect` SHALL match an entry of `auth.sso_app_redirects` exactly, `code_challenge` SHALL be required and S256 only, and `app_state` SHALL be at most 256 characters; otherwise the start SHALL be refused on the sign-in error screen. The values SHALL be sealed into the flow.

#### Scenario: Redirect off the list

- **WHEN** the app opens `/login/sso/kc` with an `app_redirect` that only begins with a listed entry
- **THEN** the browser gets the sign-in error screen and is not redirected

#### Scenario: Feature off

- **WHEN** `auth.sso_app_redirects` is empty and the app starts a sign-in
- **THEN** it is refused as turned off

### Requirement: An app sign-in hands a one-time code to the app

Where the web flow would set its session, an app flow SHALL set no session cookie and SHALL store a code of at least 32 random bytes, hashed, bound to the challenge, the account, the tenant and the provider, valid two minutes, and send the browser to `app_redirect` with `code` and `state`. The provider, the linking rules and the second factor SHALL run as on the web.

#### Scenario: OIDC

- **WHEN** an app signs in through an OIDC provider
- **THEN** the callback answers 302 to `red.tix.mobile:/sso?code=CODE&state=STATE` without a session cookie

#### Scenario: SAML after a POST

- **WHEN** an app signs in through a SAML provider
- **THEN** the ACS answers a page that navigates to and links the app's redirect with the code

### Requirement: The second factor of an app sign-in is asked in the browser

An app sign-in SHALL ask the tenant's second factor in the browser when the tenant requires it on the web or on the command line, or the person holds one, and SHALL hand over the code only once it passed, showing any recovery codes first.

#### Scenario: Cli requirement

- **WHEN** the tenant requires a second factor on the command line only and an app signs in
- **THEN** the browser asks for it before the code is handed over

### Requirement: A refused app sign-in returns to the app

Once `app_redirect` was validated, a refused sign-in SHALL send the browser to `app_redirect` with `error` set to one of `access_denied`, `not_linked`, `expired`, `provider_error` or `server_error` and the app's `state`, and no code.

#### Scenario: Not linked

- **WHEN** the provider vouches for an email no account here holds
- **THEN** the browser goes to the app's redirect with `error=not_linked`

### Requirement: The app exchanges the code once

`POST /api/v1/auth/sso/exchange` SHALL take `code` and `code_verifier` without a credential and answer as a completed `POST /api/v1/auth/login` does, with a remembered session of method `sso:KEY` and a `user.login` audit entry. An unknown, used or expired code or a verifier whose S256 is not the challenge SHALL be 401 invalid credentials, logged as `sso_code_invalid`. A code SHALL be taken once across replicas.

#### Scenario: Wrong verifier

- **WHEN** an exchange sends the right code with another verifier
- **THEN** it is 401 and the code cannot be exchanged afterwards

#### Scenario: Race

- **WHEN** several exchanges of one code run at once
- **THEN** exactly one gets a session
