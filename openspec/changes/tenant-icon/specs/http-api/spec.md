## ADDED Requirements

### Requirement: Tenant icon endpoint

A tenant read, each `whoami` membership and `whoami`'s own tenant SHALL carry the tenant's icon
fields as present (`icon`, `icon_name`, `icon_image`; on the actor `tenant_icon`,
`tenant_icon_name`, `tenant_icon_image`). `PATCH /api/v1/tenants/{ref}` SHALL accept `icon`,
`icon_name` and `icon_file` as a project update does. The API SHALL expose
`GET /api/v1/tenants/{ref}/icon` answering with the image as the project icon route does.

#### Scenario: Upload and fetch over HTTP

- **WHEN** an admin patches the tenant with `icon_file` holding a PNG and then requests `GET /api/v1/tenants/{ref}/icon`
- **THEN** the patch answers with a tenant carrying `icon_image`, and the fetch answers 200 with the same bytes and `Content-Type: image/png`

### Requirement: Any member reads a tenant's icon

`GET /api/v1/tenants/{ref}/icon` SHALL answer for the caller's own tenant, for any tenant to a
global admin, and for any other tenant the caller's account belongs to, a token only when it
carries `account:read`. Any other tenant, and one without an uploaded image, SHALL be not found.

#### Scenario: A member reads the icon of their other tenant

- **WHEN** a person signed in to tenant A requests the icon of tenant B, which they also belong to
- **THEN** the response is 200 with B's image

#### Scenario: A stranger cannot read a tenant's icon

- **WHEN** a person who does not belong to tenant B requests its icon
- **THEN** the response is 404 in the error envelope
