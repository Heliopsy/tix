## ADDED Requirements

### Requirement: Move is a registered capability

The capability registry SHALL hold `task.move` bound to `MoveTask` with the `task:write` scope and bindings
on the CLI, HTTP, web and terminal surfaces, and the transport matrix SHALL exercise it.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** `MoveTask` appears exactly once with a CLI command, an HTTP route, a web route and a TUI view
