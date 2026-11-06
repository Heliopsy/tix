## ADDED Requirements

### Requirement: The global admin area

The browser SHALL offer a Global admin entry only to a global admin. Its screen SHALL list every tenant
with create, rename and delete in dialogs, the global admins with grant and revoke, and link each tenant to
a screen managing its users.

#### Scenario: A tenant admin's sidebar

- **WHEN** a tenant admin who is not a global admin opens any screen
- **THEN** the sidebar has no Global admin entry and `/admin/global` answers 403

### Requirement: Generated passwords are shown once

After a tenant is created from the browser, the next load of the global admin screen SHALL show each
generated password with its email and a copy button, and a reload SHALL show none.

#### Scenario: Reloading

- **WHEN** the screen showing the passwords is loaded again
- **THEN** none of the passwords appears
