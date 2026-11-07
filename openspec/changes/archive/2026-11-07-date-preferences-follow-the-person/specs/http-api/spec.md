## ADDED Requirements

### Requirement: The caller's date settings over HTTP

`GET /api/v1/me/time` SHALL return the deployment, tenant and personal levels, the values in force and the
level each came from. `PATCH /api/v1/me/time` SHALL set the caller's own `time_format` and `timezone`,
where `""` inherits and an absent field is left alone. A caller with no user record SHALL be refused
`invalid` by the `PATCH`.

#### Scenario: An agent cannot hold a preference

- **GIVEN** a token held by an agent
- **WHEN** it sends `PATCH /api/v1/me/time` with `{"time_format": "us"}`
- **THEN** the response is `invalid` and nothing is stored

#### Scenario: Unknown values are refused

- **WHEN** a person sends `{"timezone": "Mars/Olympus"}` or `{"timezone": "local"}`
- **THEN** the response is `invalid`

### Requirement: A tenant's default date settings over HTTP

`PATCH /api/v1/tenants/REF` SHALL accept `time_format` and `timezone`, where `""` returns the field to the
deployment default.

#### Scenario: Tenant default set

- **WHEN** an administrator sends `{"time_format": "dotted"}`
- **THEN** members who chose nothing read dotted timestamps
