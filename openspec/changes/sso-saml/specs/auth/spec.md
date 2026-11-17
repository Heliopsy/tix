## MODIFIED Requirements

### Requirement: A tenant configures its own sign-in providers

A tenant SHALL hold any number of sign-in providers, each with a key unique in the tenant, a kind of `oidc`, `oauth_github` or `saml`, a display name and an enabled flag. Creating, changing, listing and deleting providers SHALL require the `sso:admin` scope, which the admin role carries.

#### Scenario: Member refused

- **WHEN** a member lists the tenant's providers
- **THEN** the call is forbidden

#### Scenario: SAML accepted

- **WHEN** an admin creates a provider of kind `saml` with an IdP entity id, sign-in URL and signing certificate
- **THEN** it is stored and offered on the sign-in screen

## ADDED Requirements

### Requirement: A SAML provider is configured from IdP metadata

A SAML provider SHALL hold the IdP's entity id, its HTTP-Redirect sign-in URL and at least one signing certificate. They SHALL be readable from the IdP's metadata, given by URL or as XML, with any value given explicitly taking precedence. A provider without all three, or with a certificate that does not parse, SHALL be refused naming the field.

#### Scenario: Metadata by URL

- **WHEN** an admin adds a SAML provider naming only the IdP's metadata URL
- **THEN** the entity id, sign-in URL and certificates are read from it and stored

#### Scenario: No certificate

- **WHEN** an admin adds a SAML provider with no signing certificate
- **THEN** it is refused naming `idp_certificates`

### Requirement: A SAML sign-in starts at tix

A SAML sign-in SHALL begin with an AuthnRequest sent over HTTP-Redirect, whose ID is kept in the sealed flow and whose RelayState is the flow's state. The response SHALL be posted to the provider's assertion consumer service, and the flow SHALL travel in a cookie that survives that cross-site POST over TLS.

#### Scenario: Begin

- **WHEN** a person chooses Sign in with a SAML provider
- **THEN** the browser is sent to the IdP's sign-in URL carrying a SAMLRequest and a RelayState

### Requirement: A SAML response is checked before it names anybody

A SAML response SHALL be refused unless its assertion is signed by a configured certificate or lies in a signed response, its issuer is the IdP's entity id, its audience, recipient and destination are this provider's, it is inside NotBefore and NotOnOrAfter, and it carries exactly one assertion. An unsigned response SHALL be refused.

#### Scenario: Bad signature

- **WHEN** the response is signed by a key that is not one of the provider's certificates
- **THEN** the sign-in is refused and no session is minted

#### Scenario: Wrong audience

- **WHEN** the assertion's audience is another service provider
- **THEN** the sign-in is refused

#### Scenario: Expired

- **WHEN** the assertion's NotOnOrAfter has passed
- **THEN** the sign-in is refused

#### Scenario: Unsigned

- **WHEN** neither the response nor its assertion is signed
- **THEN** the sign-in is refused

#### Scenario: Signature wrapping

- **WHEN** an unsigned assertion is placed beside or around a signed one
- **THEN** the sign-in is refused and the unsigned assertion names nobody

### Requirement: A SAML response answers this browser's request once

The signed InResponseTo SHALL match the AuthnRequest of the flow in the browser's cookie, and the RelayState SHALL match the flow's state. An assertion id SHALL be accepted once per provider until it expires, on every replica.

#### Scenario: Replayed

- **WHEN** a response that already signed somebody in is posted again
- **THEN** it is refused as already used

#### Scenario: Other request

- **WHEN** the response answers an AuthnRequest other than the flow's
- **THEN** it is refused

### Requirement: IdP-initiated SAML is off by default

A response that answers no request SHALL be refused unless the provider allows IdP-initiated sign-in. When allowed, it SHALL carry no InResponseTo and SHALL pass every other check.

#### Scenario: Unsolicited refused

- **WHEN** an IdP posts an unsolicited response to a provider that does not allow it
- **THEN** the sign-in is refused naming the setting

### Requirement: A SAML assertion becomes the shared identity

The NameID SHALL be the subject, and a transient NameID SHALL be refused. Email, name, username and groups SHALL be read from attributes named per provider, by Name or FriendlyName, with defaults covering common IdPs. The identity SHALL then go through the same linking, creation, role mapping, second factor and session as every other kind.

#### Scenario: Linked like OIDC

- **WHEN** a SAML assertion names the email of a member of the tenant
- **THEN** the member is linked by NameID and signed in with method `sso:KEY`

### Requirement: Each SAML provider publishes SP metadata

Each SAML provider SHALL publish service provider metadata without a credential, naming its entity id and assertion consumer service and asking for signed assertions.

#### Scenario: Metadata

- **WHEN** an IdP administrator fetches `/login/sso/saml/KEY/metadata`
- **THEN** the answer is SAML metadata with the provider's entity id and ACS address
