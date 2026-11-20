## ADDED Requirements

### Requirement: Sign-in options say whether an app may sign in

`GET /api/v1/auth/sso/options` SHALL answer `app_sign_in`, true when `auth.sso_app_redirects` is not empty, beside `password_login` and `providers`.

#### Scenario: Default

- **WHEN** a server with the default configuration is asked for its sign-in options
- **THEN** `app_sign_in` is true
