## ADDED Requirements

### Requirement: The channel is stored in columns and JSON

Migration 0033 SHALL add via_kind defaulting to unknown, via_token and via_detail to audit_entries, and via_kind and via_detail to events, with an index on audit_entries over tenant, via_kind and seq, on both engines.

#### Scenario: Upgrading a database

- **WHEN** a database holding audit entries migrates to 0033
- **THEN** every existing entry reads as unknown and new entries carry their channel
