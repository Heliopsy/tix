## ADDED Requirements

### Requirement: Copy an audit entry as JSON

Every audit row on Activity, on an actor's screen and in a task's History SHALL carry a Copy JSON button that copies the row's audit entry as the API returns it, pretty-printed and embedded in the page. It SHALL confirm with Copied, fall back to the legacy copy command where the Clipboard API is missing, and never carry a token secret.

#### Scenario: Copying a row

- **WHEN** a reader presses Copy JSON on an activity row
- **THEN** the clipboard holds that row's audit entry as indented JSON
- **AND** the button reads Copied for a moment
