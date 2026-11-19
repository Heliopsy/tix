## ADDED Requirements

### Requirement: tix tenant sso refresh rereads a SAML provider's metadata

`tix tenant sso refresh KEY` SHALL reread the provider's IdP metadata from its stored URL and print the provider. It SHALL exit 7 when the metadata cannot be read, 2 when it is not valid, and 6 for a provider with no metadata URL. `tix tenant sso show` SHALL show the last read's time, outcome and error.

#### Scenario: Refresh after a rotation

- **WHEN** `tix tenant sso refresh okta` runs after the IdP published a new certificate
- **THEN** the provider printed holds both certificates and its last read is `updated`

#### Scenario: Unreachable IdP

- **WHEN** the metadata URL cannot be read
- **THEN** the command exits 7 and `tix tenant sso show okta` shows the failure
