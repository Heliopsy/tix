## ADDED Requirements

### Requirement: Recurrence travels in a snapshot

A tenant snapshot SHALL carry each task's recurrence rule and successor link, and importing it SHALL store
both, so that an export followed by an import reproduces them. An import SHALL re-validate the rule and
reject a record whose rule does not parse, and SHALL NOT create any occurrence, whatever state the imported
tasks are in.

#### Scenario: Round trip

- **WHEN** a tenant with a task carrying `every 2 weeks on mon` and a finished task linked to its successor is exported and imported into an empty tenant
- **THEN** the imported tasks carry the same rule and the finished one still records a successor
- **AND** no task beyond those in the snapshot is created

#### Scenario: Invalid rule in a snapshot

- **WHEN** a snapshot task carries the rule `every fortnight`
- **THEN** that record is rejected with a message naming the rule
