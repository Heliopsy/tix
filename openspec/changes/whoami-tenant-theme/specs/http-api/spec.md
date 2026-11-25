## ADDED Requirements

### Requirement: whoami names the tenant for every credential

`GET /whoami` SHALL report `tenant_key` and `tenant_name` of the caller's tenant for every credential: a session, an SSH sign-in, an API token whoever it was minted for, and an agent's token. Memberships SHALL keep their own rule and be listed only to a session or a token carrying `account:read`.

#### Scenario: An agent's token

- **WHEN** a token minted for an agent, carrying only `task:read`, calls whoami
- **THEN** the answer carries the tenant's key and name and lists no memberships

### Requirement: whoami carries the tenant's resolved theme

`GET /whoami` SHALL carry `theme`, the palette the web interface renders for the caller's tenant, with `name`, `accent`, `accent_soft`, `accent_dark` and `accent_soft_dark` as `#rrggbb` and `built_in`. The dark pair SHALL be filled from the light pair when the theme names none. No other tenant setting SHALL be added to the answer.

#### Scenario: A member without tenant:admin

- **WHEN** a token carrying only `task:read` calls whoami in a tenant that names the `ocean` theme
- **THEN** the answer's theme is `ocean` with that theme's four colours

#### Scenario: A configured theme with no dark pair

- **WHEN** the tenant names a configured theme defining only `accent` and `accent_soft`
- **THEN** whoami answers `accent_dark` and `accent_soft_dark` equal to them
