## ADDED Requirements

### Requirement: The sign-in screen offers each enabled provider

The sign-in screen SHALL show a "Sign in with NAME" button for each enabled provider of the tenant, carrying Keep me signed in and the screen to return to. When password sign-in is disabled the password form SHALL be replaced by a note that the tenant signs in through its providers.

#### Scenario: Provider button

- **WHEN** a tenant has an enabled provider named Keycloak
- **THEN** the sign-in screen shows Sign in with Keycloak

#### Scenario: Password disabled

- **WHEN** the tenant disabled password sign-in
- **THEN** the sign-in screen has no password field

### Requirement: A Single sign-on screen manages providers

Tenant settings SHALL have a Single sign-on screen listing the providers, with a create action opening a dialog, an edit dialog per provider, a red delete with a confirm dialog, and the password sign-in setting. It SHALL never show a stored client secret.

#### Scenario: Create from the screen

- **WHEN** an admin fills in the new provider dialog and submits it
- **THEN** the provider is listed and its secret is shown only as set
