## ADDED Requirements

### Requirement: A help button on every screen

Every screen of the browser interface SHALL carry a help link at the right end of its header row,
named "Help: " followed by the screen's title and opening in a new tab with `rel="noopener"`. It
SHALL lead through `/docs/{page}` to the documentation section about that screen, for the running
release. The task screen drawn in the task dialog SHALL carry none.

#### Scenario: The task list's help

- **GIVEN** a reader on `/tasks`
- **WHEN** they follow the help link in the header
- **THEN** it opens `/docs/web-ui.md?section=the-task-list` in a new tab, which redirects to that page
  of the running release with `#the-task-list`

### Requirement: The docs redirect carries a section

`GET /docs/{page}` SHALL accept a `section` query parameter and SHALL append it to the redirect target
as a fragment when it consists of lower-case letters, digits, hyphens and underscores. Any other value
SHALL be ignored, and the redirect SHALL go to the page alone.

#### Scenario: A malformed section is dropped

- **GIVEN** a request for `/docs/web-ui.md?section=Bad%20Section`
- **WHEN** the server answers
- **THEN** it redirects to the page with no fragment
