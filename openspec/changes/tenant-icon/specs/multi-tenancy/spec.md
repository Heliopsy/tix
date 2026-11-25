## ADDED Requirements

### Requirement: A tenant's icon is visible to every member

A tenant's icon SHALL be readable by every member of the tenant from a credential in any tenant
they belong to, so a switcher can draw every tenant it lists, and by nobody else but a global
admin. It SHALL be set only by whoever may change the tenant's settings.

#### Scenario: A viewer cannot set the icon

- **WHEN** a viewer of a tenant sends an update naming `icon`
- **THEN** it is refused as forbidden and the icon is unchanged
