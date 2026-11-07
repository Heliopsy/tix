## ADDED Requirements

### Requirement: Create a user by password or by invite link

The Users screen's create dialog SHALL offer a choice between Set a password and Send an invite
link, both visible controls rather than a fold. With a password, a Require a password change at
first sign-in checkbox SHALL be ticked by default. An invite SHALL answer with a dialog showing the
link once with a Copy button and the words send this link to them.

#### Scenario: Inviting from the browser

- **WHEN** an administrator chooses Send an invite link and submits an email
- **THEN** a dialog shows the link with a Copy button, and the Pending invites section lists the address

### Requirement: Pending invites on the Users screen

The Users screen SHALL list pending invites with their address, role, inviter and expiry, each with
a Re-issue button and a red Revoke button opening a confirm dialog. The Manage dialog SHALL carry the
require password change checkbox.

#### Scenario: Revoking from the list

- **WHEN** an administrator confirms Revoke on a pending invite
- **THEN** the invite leaves the list and its link stops working

### Requirement: Invite and password change pages

`/invite/TOKEN` SHALL show a form to choose a password and a display name, or to confirm an existing
password, and sign the person in. A flagged sign-in SHALL show a Choose a new password page whose
only other action returns to sign-in; nothing else is reachable because no session exists.

#### Scenario: Forced change in the browser

- **GIVEN** a flagged account
- **WHEN** its owner signs in on the web
- **THEN** the Choose a new password page is shown, and opening the projects list sends them to sign-in
