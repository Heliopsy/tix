## ADDED Requirements

### Requirement: Tenant artifact limits only tighten

A tenant administrator SHALL be able to set the tenant's own artifact size limit and accepted types. A size above
the server's or a type the server does not accept SHALL be refused with a message naming the server's value. Zero
and an empty list SHALL inherit the server's. The effective limits SHALL be the tighter of the tenant's and the
server's at the time of each upload.

#### Scenario: Above the server

- **WHEN** a tenant sets its artifact size limit to 5 MiB on a server whose limit is 1 MiB
- **THEN** the change is refused with a message naming 1 MiB

#### Scenario: Cleared

- **WHEN** a tenant sets its artifact types to an empty list
- **THEN** the server's accepted types apply to it
