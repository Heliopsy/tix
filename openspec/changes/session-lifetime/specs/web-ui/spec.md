## ADDED Requirements

### Requirement: Keep me signed in on the sign-in screen

The sign-in screen SHALL show a Keep me signed in checkbox, stating how long it lasts, when the tenant offers
it, and SHALL not draw it otherwise. The choice SHALL ride through the second factor and password change steps.

#### Scenario: Offered

- **WHEN** a signed-out browser opens `/login` on a tenant with the default settings
- **THEN** the form carries a Keep me signed in checkbox

#### Scenario: Turned off

- **WHEN** the tenant turned Keep me signed in off
- **THEN** the sign-in form carries no such checkbox

### Requirement: Session settings on Tenant settings

Tenant settings SHALL carry Sign-in lasts, Sign out when idle for, and Keep me signed in lasts, each empty to
use the server's value shown greyed. The sessions list on Settings SHALL mark a remembered session.

#### Scenario: Saved off

- **WHEN** an administrator saves Keep me signed in lasts as off
- **THEN** Tenant settings shows off in that field
