## ADDED Requirements

### Requirement: Listing the inbox by kind

Listing the inbox SHALL accept a set of kinds and return only items of those kinds, every kind when the set
is empty. Kind names SHALL be read without regard to case or surrounding space, and an unknown kind SHALL be
refused as invalid.

#### Scenario: Only mentions

- **GIVEN** bob has an assignment, a mention and a comment item
- **WHEN** bob lists his inbox with kinds `mention`
- **THEN** only the mention is returned

#### Scenario: An unknown kind

- **WHEN** bob lists his inbox with kinds `likes`
- **THEN** the call fails as invalid
