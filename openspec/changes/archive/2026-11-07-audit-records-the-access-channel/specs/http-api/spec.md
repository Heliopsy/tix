## ADDED Requirements

### Requirement: Audit routes filter by via and token

GET /api/v1/audit and the task audit route SHALL accept repeatable via and token query parameters, and every audit entry they return SHALL carry via.

#### Scenario: Filtering by channel

- **WHEN** GET /api/v1/audit?via=mcp is requested
- **THEN** only entries recorded over MCP are returned, each with its via

### Requirement: Clients name themselves in X-Tix-Client

The tix client SHALL send X-Tix-Client: cli on every request, and tix mcp SHALL send X-Tix-Client: mcp. The server SHALL read the header only to refine the channel of a token request.

#### Scenario: tix mcp is told apart

- **WHEN** tix mcp creates a task with a token the CLI also uses
- **THEN** the MCP change records mcp and the CLI change records cli-remote
