## ADDED Requirements

### Requirement: Pages name the build that drew them

Every page of the web interface SHALL carry the server's release version on its `<body>` element as
`data-build`, the same value `GET /healthz` reports as `version`.

#### Scenario: The body names the release

- **GIVEN** a server running v0.55.0
- **WHEN** a signed-in reader opens `/tasks`
- **THEN** the page's `<body>` carries `data-build="v0.55.0"`

### Requirement: An open tab reloads into a new release when nothing would be lost

With scripting on, a page SHALL ask `/healthz` for the version when its live socket reconnects after
a drop, when it becomes visible, and every few minutes while visible. When the version differs from
`data-build` and nothing on the page would be lost, it SHALL reload itself, keep its scroll position
and then show a short status note naming the new version.

#### Scenario: A quiet tab follows a deploy

- **GIVEN** a tab showing the board, drawn by v0.55.0, with no field touched
- **WHEN** the server is upgraded to v0.56.0 and the live socket reconnects
- **THEN** the tab reloads, returns to the same scroll offset and shows "Updated to v0.56.0"

#### Scenario: The same version changes nothing

- **WHEN** the socket reconnects and `/healthz` reports the version the page was drawn by
- **THEN** the page does not reload and shows nothing

### Requirement: A tab holding work offers the reload instead

A reload SHALL NOT happen on its own while focus is in an editable field, a field has been changed
and not submitted, a dialog or popover is open, a request is in flight, or rows are selected in
select mode. The page SHALL instead show a non-blocking bar saying a new version is available, with
a Reload button, and SHALL reload by itself once none of those holds.

#### Scenario: A half-written comment is kept

- **GIVEN** a reader has typed into a comment box and not sent it
- **WHEN** the tab learns the server runs a newer version
- **THEN** the page does not reload, the text stays, and a bar offers Reload

#### Scenario: Sending the comment lets the reload through

- **GIVEN** the bar is showing because of an unsent comment
- **WHEN** the reader sends the comment and the request settles
- **THEN** the page reloads into the new version

#### Scenario: Reload on demand

- **WHEN** the reader presses Reload in the bar
- **THEN** the page reloads at once
