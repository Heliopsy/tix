## ADDED Requirements

### Requirement: The tenant settings screen sets the tenant's icon

The tenant settings screen SHALL offer the project icon picker (Emoji, Icon, Upload and No icon)
for the tenant's icon, and saving it SHALL update the tenant's icon. A form that names no icon
kind SHALL leave the icon as it is.

#### Scenario: An admin picks a built-in icon

- **WHEN** a tenant admin chooses Icon, picks rocket and saves the tenant settings
- **THEN** the tenant carries `icon_name` `rocket`

### Requirement: The switcher draws each tenant's icon

Each row of the tenant switcher and of the Choose a tenant screen SHALL draw the tenant's icon,
or its monogram when it carries none, and the sidebar entry opening the switcher SHALL draw the
current tenant's. A tenant's own icon SHALL replace the logo beside its name in the header. An
uploaded image SHALL be drawn from `/tenants/{key}/icon`, which any member of the tenant reads.

#### Scenario: Two tenants, one with an icon

- **WHEN** a person belongs to tenant `acme`, whose icon is the emoji 🚀, and tenant `beta`, named `Beta works`, with no icon
- **THEN** the switcher draws 🚀 beside Acme and `BW` beside Beta works
