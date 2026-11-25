## ADDED Requirements

### Requirement: The settings screen lists the reader's sessions

The settings screen SHALL draw a headed Sessions section listing the reader's own sessions with when each began and expires, how it signed in and its user agent, marking the one in use. Each session SHALL carry a red Revoke button that opens a confirm dialog holding the form.

#### Scenario: Revoking another session from the browser

- **WHEN** a person confirms Revoke on a session that is not the one in use
- **THEN** that session ends and the settings screen comes back without it
