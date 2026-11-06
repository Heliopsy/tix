## ADDED Requirements

### Requirement: A create action is a header button that opens a dialog

A screen whose action makes a record SHALL offer it as a primary button in the page header, or on the
right of the heading of the section it adds to, and SHALL NOT fold the form at the foot of the screen.
The button SHALL open the form in a modal dialog with a Cancel control. An empty list SHALL carry the
same button.

#### Scenario: Issuing a token starts from the header

- **WHEN** the token screen is read
- **THEN** its header holds an Issue a token button
- **AND** the form it opens is in a dialog, not under the list

### Requirement: A create dialog works without scripting and after a refusal

Without scripting a create button SHALL be a link to the same screen with the dialog drawn open in
place. A submission the service refuses as correctable SHALL come back to that screen with the dialog
open, the message inside it beside its field and the values typed kept.

#### Scenario: A refused user comes back in its dialog

- **WHEN** a user is created with an invalid email
- **THEN** the users screen is drawn with the Create a user dialog open
- **AND** the message and the typed display name are inside it

#### Scenario: The fallback address draws the form

- **WHEN** the token screen is read at `?dialog=token-new`
- **THEN** the Issue a token form is drawn open in place

### Requirement: A project's settings are a screen of their own

A project's settings SHALL be the screen `/projects/{key}/settings`, reached from a Settings button in
the board's header, holding Project, Claims, Housekeeping and, last, Danger zone as always-drawn headed
sections. The board SHALL hold the columns and none of these forms.

#### Scenario: The board links to its settings

- **WHEN** a project board is read
- **THEN** its header carries a Settings button to `/projects/{key}/settings`
- **AND** the board carries no settings, claims, housekeeping or archive form

### Requirement: A destructive action is a red button and a confirm dialog

An action that cannot be undone SHALL be a red button that opens a dialog saying what happens, and the
form that takes the step SHALL be only in that dialog. Revoking a token, deleting a user or a template,
removing a notification channel, a project's Clean up now and archiving a project SHALL work this way.

#### Scenario: Deleting a template asks first

- **WHEN** a template's screen is read
- **THEN** Delete template is a red button opening a dialog
- **AND** the delete form is inside the dialog and nowhere else

### Requirement: No page action, setting or explanation is folded

A template SHALL NOT draw a page action, a page's settings or an explanation inside a `<details>`
element. A toggle on one row or one field MAY stay one, and a test SHALL name each such toggle with
its reason.

#### Scenario: A new fold fails the build

- **WHEN** a template adds a `<details>` that is not a listed toggle
- **THEN** the template guard fails

## MODIFIED Requirements

### Requirement: Revoking a token is confirmed first and never offered for a revoked one

A revoked token SHALL NOT offer a revocation control. Revocation, being irreversible and ending a
credential something may be holding, SHALL be behind a confirm dialog that names the token it ends
and says what revoking does.

#### Scenario: A revoked token is shown as revoked and offers no revocation

- **GIVEN** a revoked token
- **WHEN** the token listing is read
- **THEN** the row is marked revoked
- **AND** the row offers no control to revoke it

#### Scenario: Revocation is confirmed before it happens

- **GIVEN** a live token
- **WHEN** the token listing is read
- **THEN** the row's Revoke button opens a dialog that names the token and says that whatever holds it
  stops authenticating

### Requirement: A token revocation control is offered only for a token the reader is shown

A revocation control SHALL be offered only for a token the reader is shown, and the control for a token
held by another actor SHALL say whose it is before it is used. No control SHALL be offered for a token the
reader may not see.

#### Scenario: The control for somebody else's token says whose it is

- **GIVEN** an administrator reading a token held by another actor
- **WHEN** the revocation dialog for that row is opened
- **THEN** it says the token is not the reader's own
- **AND** the button naming the token also names its owner

#### Scenario: An administrator revokes another actor's token from the browser

- **GIVEN** an administrator and a live token held by another actor
- **WHEN** the administrator submits that row's revocation
- **THEN** the token is revoked
- **AND** the row comes back marked revoked and offering no revocation control

### Requirement: Configure screens hold together at every width

The configuration and data screens SHALL draw without the page scrolling sideways at viewport widths
of 1440px, 900px and 390px. A table that cannot fit SHALL scroll inside its own frame, a row button's
label SHALL NOT break between its letters, and an information note SHALL stay inside the viewport.

#### Scenario: The token table with Revoke open

- **GIVEN** a viewport 900px wide and a tenant with API tokens
- **WHEN** the reader opens a token's Revoke dialog
- **THEN** the page is no wider than the viewport
