## ADDED Requirements

### Requirement: The SAML metadata refresh interval is configurable

`auth.saml_metadata_refresh` (`TIX_AUTH_SAML_METADATA_REFRESH`) SHALL set how often `tix serve` rereads SAML IdP metadata, defaulting to `12h`. `0` SHALL turn the scheduled read off and a negative value SHALL be refused naming the key.

#### Scenario: Off

- **WHEN** `auth.saml_metadata_refresh` is `0`
- **THEN** no scheduled read runs and a provider shows no next read

#### Scenario: Negative

- **WHEN** `auth.saml_metadata_refresh` is `-1h`
- **THEN** the configuration is refused naming `auth.saml_metadata_refresh`
