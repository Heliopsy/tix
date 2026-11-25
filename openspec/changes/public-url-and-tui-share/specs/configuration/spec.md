## ADDED Requirements

### Requirement: Public web address

The configuration SHALL accept `server.public_url` (`TIX_SERVER_PUBLIC_URL`), the address people reach
the browser interface at. It SHALL change nothing about where the server binds. Unset, every link
SHALL be built as it was before the key existed.

#### Scenario: The address is shown, not bound

- **GIVEN** `TIX_SERVER_PUBLIC_URL=https://tix.example.com` and `tix serve --listen 127.0.0.1:8080`
- **WHEN** the server starts
- **THEN** it binds 127.0.0.1:8080, and a task read over the API carries `web_url`
  `https://tix.example.com/tasks/REF`

### Requirement: The public web address is validated at load

`server.public_url` SHALL be an absolute `http` or `https` address with a host and no user, query or
fragment. A path SHALL be accepted only as a base path, and a trailing slash SHALL be dropped. Any
other value SHALL fail at load, naming the key.

#### Scenario: A query is refused

- **WHEN** `TIX_SERVER_PUBLIC_URL=https://tix.example.com/?tenant=acme` is set
- **THEN** every command fails with exit 2 naming `server.public_url`

#### Scenario: A base path is kept

- **GIVEN** `server.public_url: https://example.com/tix/`
- **WHEN** a task's link is built
- **THEN** it is `https://example.com/tix/tasks/REF`
