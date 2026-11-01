## ADDED Requirements

### Requirement: Templates are registered capabilities

The capability registry SHALL hold `template.list`, `template.show`, `template.save`,
`template.update`, `template.delete` and `template.use`, each bound to its service method with
CLI, HTTP, web and terminal bindings.

#### Scenario: Registry parity

- **WHEN** the parity tests run
- **THEN** every template method appears exactly once with a binding on every surface
