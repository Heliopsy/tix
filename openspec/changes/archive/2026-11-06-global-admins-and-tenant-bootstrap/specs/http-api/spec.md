## ADDED Requirements

### Requirement: Tenant creation returns the first admins

`POST /api/v1/tenants` SHALL accept `admins`, a list of `email`, `handle`, `display_name` and `password`,
and SHALL answer 201 with the tenant's fields plus `admins` and, for generated passwords,
`initial_credentials` of `email`, `handle` and `password`.

#### Scenario: Creating with one admin

- **WHEN** a global admin posts a tenant with one admin and no password
- **THEN** the answer is 201 and `initial_credentials` holds one password for that email

### Requirement: Global admin routes

The API SHALL serve `GET` and `POST /api/v1/global-admins`, `DELETE /api/v1/global-admins/{email}`,
`GET` and `POST /api/v1/tenants/{ref}/users`, and `PATCH` and `DELETE /api/v1/tenants/{ref}/users/{id}`,
all for a global admin only.

#### Scenario: A tenant admin's token

- **WHEN** a token holding `*` but not `global:admin` lists `/api/v1/tenants/other/users`
- **THEN** the answer is 403
