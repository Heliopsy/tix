## ADDED Requirements

### Requirement: SAML IdP metadata is reread from the stored URL

The system SHALL read again, from the URL stored on the provider, the IdP metadata of a SAML provider that has a metadata URL, on a schedule for enabled providers and on demand by an admin holding `sso:admin`. A disabled provider, or one configured from a document or by hand, SHALL NOT be read on the schedule.

#### Scenario: Scheduled read

- **WHEN** an enabled SAML provider with a metadata URL was last read longer ago than the refresh interval
- **THEN** the server reads its metadata again

#### Scenario: Disabled provider skipped

- **WHEN** a SAML provider with a metadata URL is disabled
- **THEN** the schedule does not read it

#### Scenario: Member refused

- **WHEN** a member asks to refresh a provider's metadata
- **THEN** the call is forbidden

### Requirement: Refreshed certificates roll over with an overlap

A refresh SHALL trust at once every signing certificate the metadata names. A certificate the provider holds that the metadata no longer names SHALL keep verifying for seven days from the first refresh that found it gone, and SHALL then be dropped.

#### Scenario: New certificate published

- **WHEN** the metadata names a new signing certificate beside the current one
- **THEN** a response signed by either is accepted after the refresh

#### Scenario: Old certificate dropped after the overlap

- **WHEN** the metadata stopped naming a certificate more than seven days ago
- **THEN** the next refresh removes it from the provider

### Requirement: A failed refresh changes nothing

A refresh whose read fails, or whose metadata does not parse, names no signing certificate or would leave the provider invalid, SHALL leave the provider's IdP settings unchanged, SHALL record the failure and its reason on the provider, and SHALL be tried again after an hour when the interval is longer.

#### Scenario: IdP unreachable

- **WHEN** the metadata URL answers an error
- **THEN** the provider keeps its certificates, sign-in with them still works, and the provider shows the failure

#### Scenario: Invalid metadata

- **WHEN** the metadata URL answers a document that is not SAML metadata
- **THEN** the provider keeps its certificates and records the failure

### Requirement: Settings an admin set win over a refresh

The IdP entity id, sign-in URL and certificates an admin gave explicitly SHALL be recorded as pinned, and a refresh SHALL leave a pinned setting unchanged. A value given equal to the stored one SHALL NOT pin it, and a value given empty SHALL hand the setting back to the metadata.

#### Scenario: Pinned sign-in URL

- **WHEN** a provider's sign-in URL was given explicitly and the metadata names another
- **THEN** a refresh keeps the given URL and still merges the certificates

### Requirement: A changing refresh is audited

A refresh that changes a provider's certificates or sign-in URL SHALL write an `sso.provider.refresh` audit entry and an `sso.provider.updated` event in the same transaction. A refresh that changes nothing SHALL record only its outcome and SHALL NOT change the provider's `updated_at`.

#### Scenario: Rotation audited

- **WHEN** a refresh adds a certificate
- **THEN** an `sso.provider.refresh` audit entry names the provider and the count added
