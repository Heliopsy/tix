## ADDED Requirements

### Requirement: auth.mode oidc points to per-tenant providers

`auth.mode` SHALL accept `token`. `oidc` SHALL be refused with a message saying single sign-on is configured per tenant with `tix tenant sso add`. `none` SHALL remain refused as not implemented.

#### Scenario: oidc refused

- **WHEN** `TIX_AUTH_MODE=oidc` is set
- **THEN** the configuration is refused and the message names `tix tenant sso add`
