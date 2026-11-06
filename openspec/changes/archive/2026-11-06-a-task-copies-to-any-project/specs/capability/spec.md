## ADDED Requirements

### Requirement: Copy is a registered capability

The capability registry SHALL hold `task.copy` bound to `CopyTask` with the `task:write` scope
and bindings on the CLI, HTTP, web and terminal surfaces.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** `CopyTask` appears exactly once with a CLI command, an HTTP route, a web route and a TUI view
