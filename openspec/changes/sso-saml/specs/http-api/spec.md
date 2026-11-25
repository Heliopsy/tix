## ADDED Requirements

### Requirement: SAML over the sign-in routes

`POST /api/v1/auth/sso/complete` SHALL take a `saml_response` beside the flow and state, and `GET /api/v1/auth/sso/providers/ID/saml-metadata` SHALL answer a SAML provider's SP metadata as `application/samlmetadata+xml` without a credential.

#### Scenario: Metadata over the API

- **WHEN** a client requests a SAML provider's metadata route
- **THEN** it receives the SP metadata XML
