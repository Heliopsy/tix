## ADDED Requirements

### Requirement: The account screen rotates a token

The terminal account view SHALL rotate the selected live token on `R`: it SHALL ask for a grace from the browser's choices, confirm naming the token, and show the new value once as an issued token is shown.

#### Scenario: Rotating from the account view

- **WHEN** a reader presses `R` on a live token, picks a grace and confirms
- **THEN** the service is asked to rotate that token with that grace
- **AND** the new value is shown once at the top of the screen

#### Scenario: A token that no longer works

- **WHEN** a reader presses `R` on a revoked, expired or retiring token
- **THEN** nothing is asked of the service and the screen says why
