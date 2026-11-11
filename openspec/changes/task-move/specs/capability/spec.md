## ADDED Requirements

### Requirement: Move is a registered capability

The capability registry SHALL hold `task.move` bound to `MoveTask` with the `task:write` scope and
bindings on the CLI, HTTP and web surfaces, with a written exemption for the terminal.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** `MoveTask` appears exactly once with a CLI command, an HTTP route and a web route
