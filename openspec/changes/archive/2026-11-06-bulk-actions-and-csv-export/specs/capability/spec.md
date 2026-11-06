## ADDED Requirements

### Requirement: Bulk actions are a registered capability

The capability registry SHALL hold `task.bulk` bound to `BulkTasks` with CLI, HTTP and web
bindings, recorded as varying its scope by input, and exempted from the terminal interface with
the reason that the board acts on one card at a time.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** `BulkTasks` appears exactly once with a CLI command, an HTTP route, a web route and a TUI exemption
