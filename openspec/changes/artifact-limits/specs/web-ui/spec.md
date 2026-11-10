## ADDED Requirements

### Requirement: Artifact file upload states its limits

The task screen's artifact form SHALL accept a file, SHALL state the effective size limit and accepted types next
to the file input, and SHALL set the input's `accept` attribute to the accepted types.

#### Scenario: Form drawn

- **WHEN** a member opens a task on a tenant that accepts only application/pdf up to 2 KiB
- **THEN** the artifact form says files up to 2 KiB of application/pdf are accepted and its file input carries
  `accept="application/pdf"`

### Requirement: Artifact files download only

An artifact file SHALL be served with `Content-Disposition: attachment`, `X-Content-Type-Options: nosniff` and a
sandboxing content security policy, whatever its stored type, and SHALL NOT be drawn inline.

#### Scenario: An HTML file stored before the limits

- **WHEN** a member follows the Download link of an artifact whose type is text/html
- **THEN** the browser receives it as an attachment and does not render it

### Requirement: Tenant settings edit artifact limits

Tenant settings SHALL offer the tenant's largest artifact file and accepted artifact types, each empty to inherit
the server's value, which is shown as the placeholder.

#### Scenario: Saving a tighter limit

- **WHEN** an administrator saves 2KiB as the largest artifact file
- **THEN** new uploads over 2 KiB are refused on that tenant
