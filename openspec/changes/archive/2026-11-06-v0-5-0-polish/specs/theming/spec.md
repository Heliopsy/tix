## ADDED Requirements

### Requirement: A tenant that names no theme uses the product default

A tenant with no theme name SHALL resolve to the `default` theme.

This replaces the earlier behaviour, where an unnamed tenant's accent was derived by hashing its key and
id into a small fixed palette. That kept an upgrade from repainting an existing deployment, at the cost of
a fresh install opening in a colour nobody chose, two installs of the same software looking unrelated, and
neither matching the logo.

#### Scenario: A tenant that names nothing

- **WHEN** a tenant's theme is empty or whitespace
- **THEN** its accent is the default theme's accent, whatever its key and id are

#### Scenario: Two tenants that name nothing look the same

- **WHEN** two tenants with different keys and ids both name no theme
- **THEN** they resolve to the same accent

### Requirement: The default theme is the product green

The default SHALL be the product's own green, so the colour is recognisable rather than arbitrary.

#### Scenario: The default is the product green

- **WHEN** the default theme is resolved
- **THEN** its accent is the green the logo uses

### Requirement: A tenant records a theme name, and naming none means the default

A tenant SHALL record the name of a theme. Setting a name that does not resolve SHALL be refused at the
point of writing.

When a tenant names no theme, it SHALL resolve to the default theme, as the requirement on tenants that
name no theme describes.

When a tenant names a theme that no longer resolves, reading SHALL fall back to the default theme rather
than fail.

#### Scenario: Setting an unknown theme is refused

- **WHEN** a tenant is set to a theme name that does not resolve
- **THEN** the write is refused with a not-found error naming the theme

#### Scenario: An unthemed tenant uses the default theme

- **WHEN** a tenant names no theme
- **THEN** its accent is the default theme's accent

#### Scenario: A theme that stopped being defined does not break the page

- **WHEN** a tenant names a theme and configuration no longer defines it
- **THEN** the surfaces render with the default theme and the tenant record is unchanged

## REMOVED Requirements

### Requirement: A tenant names the theme it uses

**Reason**: It said a tenant that names no theme keeps an accent derived from its own identity. An
unnamed tenant now resolves to the default theme, so the requirement is replaced rather than amended.

**Migration**: Replaced by "A tenant records a theme name, and naming none means the default", which
keeps the refusal of unknown names and the fallback for a theme that stopped being defined.
