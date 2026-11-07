## ADDED Requirements

### Requirement: An event carries its channel without network detail

Every event SHALL carry its access channel's kind, origin, token ID and name and job, and SHALL NOT carry the client address, user agent, session reference, SSH key fingerprint, host or OS user.

#### Scenario: A token event

- **WHEN** the ci-agent token creates a project
- **THEN** the project.created event names the api channel and the token and carries no client address

#### Scenario: The live tail filters by channel

- **WHEN** the terminal activity view is filtered with via:mcp
- **THEN** only events made over MCP are drawn
