## ADDED Requirements

### Requirement: The Single sign-on screen configures SAML

The Single sign-on screen SHALL offer kind `saml` in its new provider dialog with the IdP metadata, entity id, sign-in URL, certificates, SP entity id, NameID format and IdP-initiated settings, and SHALL show each SAML provider's ACS and metadata addresses.

#### Scenario: Addresses shown

- **WHEN** an admin opens the screen with a SAML provider configured
- **THEN** its row shows the ACS address and the metadata address to give the IdP

### Requirement: The assertion consumer service takes the IdP's post

`/login/sso/saml/KEY/acs` SHALL accept a cross-site form POST without the CSRF token, and SHALL finish the sign-in only through the checks of the flow cookie, the RelayState and the signed assertion.

#### Scenario: Posted response

- **WHEN** the IdP's form posts a valid SAMLResponse with the flow's RelayState
- **THEN** the browser is signed in and sent to the screen it started from
