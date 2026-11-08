## ADDED Requirements

### Requirement: Task responses carry their web address

When `server.public_url` is set, every task the service returns SHALL carry `web_url`, the absolute
address of its screen in the browser interface, over HTTP and so over MCP. It SHALL be omitted when
the key is unset, and it SHALL NOT be written into audit entries, events or exports.

#### Scenario: Set

- **GIVEN** `server.public_url: https://tix.example.com`
- **WHEN** `GET /api/v1/tasks/infra-12` is called
- **THEN** the task carries `"web_url": "https://tix.example.com/tasks/infra-12"`

#### Scenario: Unset

- **GIVEN** no `server.public_url`
- **WHEN** a task is read over HTTP or MCP
- **THEN** the response has no `web_url`
