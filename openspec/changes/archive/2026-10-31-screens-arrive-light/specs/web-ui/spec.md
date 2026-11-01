## ADDED Requirements

### Requirement: Screens and assets are compressed

The browser interface SHALL send HTML screens, fragments and static assets gzip-compressed to a
client whose `Accept-Encoding` accepts gzip, SHALL send them uncompressed otherwise, and SHALL mark
every response `Vary: Accept-Encoding`. An asset's ETag SHALL be the digest of the bytes sent in
that encoding.

#### Scenario: A browser that accepts gzip opens the task list

- **WHEN** a browser sending `Accept-Encoding: gzip, deflate, br` opens `/tasks`
- **THEN** the response carries `Content-Encoding: gzip` and is under a third of the page's size

#### Scenario: A client that refuses gzip

- **WHEN** a client sends `Accept-Encoding: gzip;q=0`
- **THEN** the page and the stylesheet arrive uncompressed

### Requirement: A form's CSRF value differs on every response

Every form on a rendered screen SHALL carry the browser's CSRF value masked with a one-time pad drawn
for that response, and the cookie's own value SHALL NOT appear in any page. A submission SHALL be
accepted when its value unmasks to the cookie's value or equals it, and refused otherwise.

#### Scenario: Two renders of the same screen

- **WHEN** the task list is rendered twice for one browser
- **THEN** the form token differs between the two, and both are accepted on a post

#### Scenario: A tampered token

- **WHEN** a form is posted with one character of its token changed
- **THEN** the post is refused as forbidden
