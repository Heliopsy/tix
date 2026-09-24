## ADDED Requirements

### Requirement: A theme is a named palette resolvable everywhere

The system SHALL provide themes as named palettes carrying an accent colour and a soft accent, and SHALL
resolve a theme by name from one registry holding both built-in themes and themes defined in configuration.
Where a name is defined in both, configuration SHALL win.

Every colour SHALL be a six-digit hexadecimal value prefixed with `#`. The system SHALL reject any other
form wherever a theme is defined or named.

#### Scenario: A built-in theme resolves by name

- **WHEN** a caller resolves a name that a built-in theme carries
- **THEN** that theme is returned with its accent and soft accent

#### Scenario: Configuration redefines a built-in name

- **WHEN** configuration defines a theme whose name matches a built-in one
- **THEN** resolving that name returns the configured palette, not the built-in one

#### Scenario: A malformed colour is refused

- **WHEN** a theme is defined with a colour that is not `#` followed by six hexadecimal digits
- **THEN** the definition is rejected and the reason names the offending colour

### Requirement: A tenant names the theme it uses

A tenant SHALL record the name of a theme. Setting a name that does not resolve SHALL be refused at the
point of writing.

When a tenant names no theme, the system SHALL derive its accent from the tenant's own identity, as it did
before any theme could be named, so that a tenant nobody has themed keeps the colour it already had.

When a tenant names a theme that no longer resolves, reading SHALL fall back to the default theme rather
than fail.

#### Scenario: Setting an unknown theme is refused

- **WHEN** a tenant is set to a theme name that does not resolve
- **THEN** the write is refused with a not-found error naming the theme

#### Scenario: An unthemed tenant keeps its derived colour

- **WHEN** a tenant names no theme
- **THEN** its accent is derived from its own identity and does not change between releases

#### Scenario: A theme that stopped being defined does not break the page

- **WHEN** a tenant names a theme and configuration no longer defines it
- **THEN** the surfaces render with the default theme and the tenant record is unchanged

### Requirement: Every surface uses the tenant's resolved theme

The web interface and the terminal interface SHALL both render a tenant's accent from the same resolved
theme, so that one tenant presents one accent on every surface.

The terminal interface SHALL continue to suppress all colour when the environment asks for none, whatever
theme is resolved. Colours that carry meaning rather than brand, being state category and priority, SHALL
NOT be replaced by the theme.

#### Scenario: One tenant, one accent

- **WHEN** a tenant names a theme and is viewed in the web interface and in the terminal interface
- **THEN** both render that theme's accent

#### Scenario: A theme does not defeat NO_COLOR

- **WHEN** the environment disables colour and the tenant names a theme
- **THEN** the terminal interface emits no colour escapes

#### Scenario: Status colours survive theming

- **WHEN** a tenant names a theme
- **THEN** state category and priority keep their own colours

### Requirement: Themes are listable

The system SHALL list every resolvable theme with its name, its palette, and whether it is built in or
configured.

#### Scenario: Listing shows both sources

- **WHEN** configuration defines a theme and the caller lists themes
- **THEN** both the built-in themes and the configured one are listed, each marked with its source
