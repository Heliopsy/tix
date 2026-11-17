## ADDED Requirements

### Requirement: SAML providers are managed under tix tenant sso

`tix tenant sso add KEY --kind saml` and `edit` SHALL take the IdP's metadata by URL or file, or its entity id, sign-in URL and certificates, plus an SP entity id, a NameID format and IdP-initiated sign-in. `tix tenant sso metadata KEY` SHALL print the provider's SP metadata. `tix login --sso` SHALL refuse a SAML provider with a pointer to the browser.

#### Scenario: Add from metadata

- **WHEN** `tix tenant sso add okta --kind saml --idp-metadata-url URL` runs
- **THEN** the provider is created with the IdP's settings read from the metadata
