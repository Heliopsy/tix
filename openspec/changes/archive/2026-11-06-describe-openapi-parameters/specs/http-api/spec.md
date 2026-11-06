## ADDED Requirements

### Requirement: Every OpenAPI parameter is described

Every query and path parameter of the OpenAPI document SHALL carry a description, and so SHALL every parameter
family an operation reads by prefix. A query parameter's description SHALL be the doc comment of its field on
the operation's query type. A parameter without one SHALL fail the build.

#### Scenario: A query field loses its doc comment

- **WHEN** a field of a query type has no doc comment and `just openapi` runs
- **THEN** the openapi tests fail naming the operation and the parameter

#### Scenario: A tool argument carries the parameter's description

- **WHEN** `tix mcp` builds the `task_list` tool
- **THEN** its `snoozed` argument's schema carries the description of `TaskListQuery.Snoozed`
