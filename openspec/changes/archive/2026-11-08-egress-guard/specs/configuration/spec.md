## ADDED Requirements

### Requirement: Egress allowances are operator keys

`server.egress_allow` SHALL list the addresses, CIDR ranges, host names and `.suffix`es a tenant-supplied
url may reach although they are internal, and `server.egress_allow_private` SHALL open every internal
address. Both SHALL default to strict, and an entry that does not parse SHALL be refused at load.

#### Scenario: An unreadable entry

- **WHEN** `TIX_SERVER_EGRESS_ALLOW` is `10.0.0.0/8,http://jira`
- **THEN** loading the configuration fails naming `server.egress_allow`

#### Scenario: An allowed tracker

- **WHEN** `server.egress_allow` is `[10.0.0.0/8]` and a tenant on `tix serve` saves a source at `http://10.1.2.3`
- **THEN** the source is saved
