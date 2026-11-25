## ADDED Requirements

### Requirement: The shell shows whether live updates get through

Every signed-in page with scripting SHALL show beside the tenant's name whether changes made elsewhere
reach it: live, connecting, reconnecting, polling or offline, as the worst of the page's live streams. A
state other than live or connecting SHALL carry its word, never only a colour, and a navigation that
reconnects SHALL NOT read as a change of state.

#### Scenario: The server goes away

- **GIVEN** a page whose live updates are live
- **WHEN** its WebSocket drops and the next try fails
- **THEN** the indicator reads Reconnecting, and Offline once a request gets no answer at all

#### Scenario: Only plain HTTP gets through

- **WHEN** the WebSocket keeps failing while the fallback poll succeeds
- **THEN** the indicator reads Polling

### Requirement: Connection details explain the live state

The indicator SHALL open connection details showing the state, what carries it, since when, the last contact
and event, failures in a row, the next try, the last error with its close code or HTTP status, hints on what
it may mean and the recent changes, with Reconnect now and Copy details. The copied report SHALL carry no
cookie, session or token.

#### Scenario: Reconnecting without waiting

- **GIVEN** a page reconnecting on a 30 second delay
- **WHEN** the reader presses Reconnect now in the connection details
- **THEN** a new connection is tried at once

#### Scenario: Copying a report

- **WHEN** the reader presses Copy details
- **THEN** a plain-text report of the details, the build and the browser is on the clipboard
