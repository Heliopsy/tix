## MODIFIED Requirements

### Requirement: A tenant that names no theme uses the product default

A tenant with no theme name SHALL resolve to the `default` theme.

This replaces the earlier behaviour, where an unnamed tenant's accent was derived by hashing its key and
id into a small fixed palette. That kept an upgrade from repainting an existing deployment, at the cost of
a fresh install opening in a colour nobody chose, two installs of the same software looking unrelated, and
neither matching the logo.

The default SHALL be the product's own green, so the colour is recognisable rather than arbitrary.

#### Scenario: A tenant that names nothing

- **WHEN** a tenant's theme is empty or whitespace
- **THEN** its accent is the default theme's accent, whatever its key and id are

#### Scenario: Two tenants that name nothing look the same

- **WHEN** two tenants with different keys and ids both name no theme
- **THEN** they resolve to the same accent

#### Scenario: The default is the product green

- **WHEN** the default theme is resolved
- **THEN** its accent is the green the logo uses
