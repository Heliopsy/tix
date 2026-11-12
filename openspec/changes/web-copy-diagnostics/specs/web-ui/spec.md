## ADDED Requirements

### Requirement: Copy diagnostics

The release screen SHALL carry a diagnostics section listing the running and newest published versions
and, with scripting, this browser's user agent, languages, time zone, screen, colour scheme, Reduce motion
and connection state, with a button copying them. The server address SHALL be included only when the
reader ticks a box for it, and no credential SHALL ever be included.

#### Scenario: Copying without the server address

- **WHEN** a reader presses Copy diagnostics with Include the server address unticked
- **THEN** the copied text names the versions and the browser and not the server's address
