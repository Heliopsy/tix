## ADDED Requirements

### Requirement: Any member can read the resolved theme

The system SHALL let any credential in a tenant read the theme the tenant resolves to, through whoami, without the `tenant:admin` scope that reading the tenant record needs. The resolution SHALL be the one every surface uses, custom accents and configured themes included.

#### Scenario: A custom accent

- **WHEN** a tenant names `#7a3ef0` and a member's token calls whoami
- **THEN** the theme carries the derived light and dark accents the browser renders

### Requirement: Configured themes resolve in the service

The service SHALL be built with the theme registry that configuration defines, so that a configured theme name can be set on a tenant from the command line and the API and resolves on whoami.

#### Scenario: Naming a configured theme from the command line

- **WHEN** configuration defines theme `acme` and an administrator runs `tix tenant edit acme --theme acme`
- **THEN** the tenant records `acme` rather than being refused as an unknown theme

### Requirement: The configuration file's themes block is read

The configuration loader SHALL read `themes` as a map from name to `accent`, `accent_soft`, `accent_dark` and `accent_soft_dark`, SHALL refuse any other field, SHALL let a per-directory file's theme replace a global one of the same name, and SHALL keep the block when a command rewrites the file.

#### Scenario: A documented themes block

- **WHEN** the configuration file defines a theme with all four colours
- **THEN** configuration loads and the theme resolves with those colours

#### Scenario: A rewrite keeps the themes

- **WHEN** a command saves a changed setting to a file that defines themes
- **THEN** the file still defines them afterwards
