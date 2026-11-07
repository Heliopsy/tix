## ADDED Requirements

### Requirement: Date preferences resolve deployment, tenant, person

The date format and the timezone a person reads SHALL each resolve from the strongest level that holds a
value: the person's own, then their tenant's default, then `output.time_format` and `output.timezone`.
An empty value at a level SHALL inherit from the level below, and the format and the zone SHALL resolve
independently.

#### Scenario: Tenant overrides the deployment

- **GIVEN** `output.time_format: iso` and a tenant default of `dotted`
- **WHEN** a member who chose nothing reads a timestamp
- **THEN** it is written dotted, and its source is the tenant default

#### Scenario: Person overrides the tenant field by field

- **GIVEN** a tenant default of `dotted` in `Europe/Sofia`
- **WHEN** a member chooses only the zone `Asia/Tokyo`
- **THEN** they read dotted timestamps in Tokyo time

#### Scenario: Inherit at every level

- **GIVEN** no tenant default and no choice of the person's own
- **WHEN** they read a timestamp
- **THEN** it follows `output.time_format` and `output.timezone`

### Requirement: Machine output stays RFC 3339

JSON, YAML, NDJSON, CSV, the HTTP API and MCP SHALL carry every timestamp as RFC 3339 in UTC, whatever any
level of the date preference says.

#### Scenario: JSON ignores a preference

- **GIVEN** a person whose format is `dotted`
- **WHEN** they run `tix task ls -o json`
- **THEN** every timestamp is RFC 3339 in UTC
