## ADDED Requirements

### Requirement: App passwords in Settings

The Settings screen SHALL carry a Tasks in other apps section with an (i) tip, showing the CalDAV address and username, the reader's app passwords with their collections, creation and last use, a Revoke button on each that opens a confirm dialog, and a primary button that opens a create dialog taking a name, the collections choice and read-only.

#### Scenario: Creating an app password in the browser

- **WHEN** a signed-in reader creates an app password named `iPhone` in Settings
- **THEN** the next screen shows the password once with a Copy button, the address and the username, and returning to Settings lists `iPhone` without the password

#### Scenario: Revoking asks first

- **WHEN** a reader presses Revoke on an app password
- **THEN** a confirm dialog opens, and confirming returns to the section with a notice and the password marked revoked
