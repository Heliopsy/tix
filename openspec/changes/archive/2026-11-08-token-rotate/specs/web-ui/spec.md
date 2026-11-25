## ADDED Requirements

### Requirement: The Tokens screen rotates a token

The Tokens screen SHALL offer a Rotate button beside Revoke on every live token, including when the listing is filtered to one actor. It SHALL open a dialog naming the token and its owner with a grace choice, and the new value SHALL be shown once with a copy control, as an issued token is.

#### Scenario: Rotating from the screen

- **WHEN** a reader rotates a token from the Tokens screen
- **THEN** the screen shows the new value once
- **AND** the old token is listed as revoked, or as retiring when a grace was chosen

#### Scenario: A retiring token offers no rotate

- **WHEN** a token is inside its grace window
- **THEN** its row offers Revoke and not Rotate
