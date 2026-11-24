## MODIFIED Requirements

### Requirement: App redirects for single sign-on

`auth.sso_app_redirects` (`TIX_AUTH_SSO_APP_REDIRECTS`) SHALL list the addresses an app sign-in may hand its result to, defaulting to `red.tix.mobile:/sso`. An empty list SHALL turn app sign-in off. An entry without a scheme, with a fragment, or with the `javascript`, `data`, `vbscript` or `file` scheme in any case SHALL be refused.

#### Scenario: Turned off

- **WHEN** the variable is set to nothing
- **THEN** sign-in options answer `app_sign_in: false`

#### Scenario: Script scheme

- **WHEN** `auth.sso_app_redirects` lists `JavaScript:alert(1)`
- **THEN** the configuration is refused naming `auth.sso_app_redirects`
