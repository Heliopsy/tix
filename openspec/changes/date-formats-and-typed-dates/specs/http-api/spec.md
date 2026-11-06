## ADDED Requirements

### Requirement: Machine interfaces read dates in one format only

The HTTP API, including `"parse": true` on task creation, and the MCP tools generated from it SHALL read
dates as `YYYY-MM-DD` or RFC 3339 only, whatever date format a deployment or a browser has chosen, because
a program must not depend on a person's preference or a browser cookie.

#### Scenario: A dotted date over HTTP

- **WHEN** a client creates a task with `"parse": true` and a title holding `due:06.11.2026`
- **THEN** the request is refused, as it is under every configured format
