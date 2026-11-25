## ADDED Requirements

### Requirement: User listing routes take include_removed

`GET /api/v1/users` and the global admin's tenant users route SHALL leave out people removed from the tenant,
and SHALL list them too when `include_removed=true`, each with `removed: true` and, on the tenant users route,
no role.

#### Scenario: Listing current users over HTTP

- **WHEN** `GET /api/v1/users` is requested in a tenant one person was removed from
- **THEN** the page does not contain that person

#### Scenario: Including removed people over HTTP

- **WHEN** `GET /api/v1/users?include_removed=true` is requested
- **THEN** the page contains that person with `removed: true`
