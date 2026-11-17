## ADDED Requirements

### Requirement: The scim access channel

The access channel vocabulary SHALL include `scim`, set by the HTTP layer for every request under `/scim/v2` together with the token's id and name, and SHALL be offered by every channel filter.

#### Scenario: Filter the trail by scim

- **WHEN** an admin runs `tix audit ls --via scim`
- **THEN** only changes made by an identity provider are listed
