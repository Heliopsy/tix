## ADDED Requirements

### Requirement: A SAML provider's metadata can be reread over HTTP

`POST /api/v1/sso/providers/{id}/refresh` SHALL reread the provider's IdP metadata and answer the provider, needing `sso:admin`. A provider SHALL carry `metadata_refresh` with `at`, `result`, `error`, `added`, `removed` and, when the server reads it on a schedule, `next`, plus `idp_certificates_retiring` and `idp_pinned`.

#### Scenario: Refresh

- **WHEN** an admin posts to a SAML provider's refresh route
- **THEN** the answer is the provider with its `metadata_refresh` dated now
