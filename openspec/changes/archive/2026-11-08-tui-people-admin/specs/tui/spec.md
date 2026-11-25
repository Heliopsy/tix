## ADDED Requirements

### Requirement: People administration view

The TUI SHALL provide a people view listing the tenant's users, each with role, state and second factor, and the pending invites, with one cursor over both lists. It SHALL be offered only to a reader who may list users.

#### Scenario: Administrator opens the view

- **WHEN** a reader holding `user:admin` presses `@`
- **THEN** the people view lists the tenant's users and pending invites

#### Scenario: Member is not offered the view

- **WHEN** a reader without `user:admin` presses `@`
- **THEN** no view opens, no user listing is requested, and neither the help overlay nor the palette offers the key

### Requirement: Creating a user or an invite from the terminal

The people view SHALL create a user with a password, optionally requiring a password change at first sign-in, or issue an invite link, from one form whose mode decides which fields are shown. A password SHALL be masked while typed and while shown.

#### Scenario: Password with forced change

- **WHEN** the reader creates a user with a password and requires a change
- **THEN** the user is created with that password and the forced change set

#### Scenario: Invite instead of a password

- **WHEN** the reader switches the form to an invite and submits it
- **THEN** an invite is issued with the chosen role and expiry and no password is sent

### Requirement: An issued invite link is shown once

The people view SHALL show the link of an invite it issued or re-issued, with its expiry and that it is not shown again, until the reader leaves the view or performs another action.

#### Scenario: Link on screen

- **WHEN** an invite is issued from the people view
- **THEN** its link is drawn on the view with a note that it is shown once

#### Scenario: Link cleared

- **WHEN** the reader leaves the people view after an invite was issued
- **THEN** the link is no longer drawn when the view is opened again

### Requirement: Editing, deleting and second factor from the terminal

The people view SHALL edit a user's display name, role, disabled state, forced password change and password, sending only the fields the reader changed, SHALL delete a user, re-issue or revoke an invite, and clear a user's second factor. Deleting, revoking and clearing SHALL each ask a confirmation naming the subject.

#### Scenario: Only the changed field is sent

- **WHEN** the reader changes only a user's role on the edit form
- **THEN** the update carries the role and no other field

#### Scenario: Clearing a second factor asks first

- **WHEN** the reader presses the second factor key on a user
- **THEN** a confirmation naming the user is shown and nothing is cleared until it is agreed to

### Requirement: Account changes refused across tenants are explained

When the service refuses a people view change with the reason `account_in_other_tenants`, the TUI SHALL say that the person also belongs to another tenant and that only they or a global admin may change their password, second factor or account.

#### Scenario: Password of a person in two tenants

- **WHEN** a tenant administrator sets the password of a person who also belongs to another tenant
- **THEN** the status line names the person and states that only they or a global admin can make that change
