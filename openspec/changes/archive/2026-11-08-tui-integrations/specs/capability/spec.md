## ADDED Requirements

### Requirement: The registry binds the integrations operations to the terminal

The capability registry SHALL bind `webhook.list`, `webhook.put`, `webhook.delete`, `webhook.deliveries`,
`webhook.redeliver`, `sync.source.list`, `sync.source.put`, `sync.source.delete`, `sync.run`, `retention.get`,
`retention.put` and `retention.prune` to the terminal integrations view, and SHALL record no terminal gap
against any of them.

#### Scenario: None of the twelve records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the twelve operations is among them
- **AND** each of them names the integrations view

### Requirement: The integrations view lowers the terminal gap count by twelve

The recorded terminal gap count SHALL fall by exactly the twelve operations the integrations view binds, so that
closing a gap and lowering the count happen in one change.

#### Scenario: The recorded count matches the gaps that remain

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records
- **AND** a gap put back fails the gate
