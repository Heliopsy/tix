## ADDED Requirements

### Requirement: The Manage dialog sends only what changed

Saving a user's Manage dialog on the Users screen SHALL send the service only the fields whose value
differs from the user as stored: display name, role, disabled and the forced password change. A save that
changes nothing SHALL make no update.

#### Scenario: Renaming a person who belongs to two tenants

- **GIVEN** a tenant admin who is not a global admin, and a person who is a member of that tenant and another
- **WHEN** the admin changes only the person's display name or role in the Manage dialog and saves
- **THEN** the change is saved and the forced password change is neither sent nor set

#### Scenario: Saving the form as drawn

- **WHEN** an admin saves a Manage dialog without changing anything
- **THEN** no user update is recorded

### Requirement: A cross-tenant refusal is shown on the Manage dialog

When the service refuses a Manage dialog change with reason `account_in_other_tenants`, the Users screen
SHALL be drawn again with that dialog open, holding what was submitted, and a message naming the person
beside the control that asked: the forced password change box, or the clear second factor part.

#### Scenario: Asking a two-tenant person to change their password

- **WHEN** a tenant admin who is not a global admin ticks the forced password change for a person who also belongs to another tenant and saves
- **THEN** the dialog comes back open with the typed values and says that only the person or a global admin can ask them to change their password, and nothing is changed
