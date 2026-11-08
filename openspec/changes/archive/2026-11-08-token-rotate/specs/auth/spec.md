## ADDED Requirements

### Requirement: An API token can be rotated

The system SHALL rotate an API token in one step: it SHALL mint a token for the same actor with the same name, scopes and project, return its value exactly once, and retire the old token in the same transaction. A token addressed by its identifier or by its live name SHALL be rotatable.

#### Scenario: Rotating replaces the token

- **GIVEN** a live token named "worker"
- **WHEN** it is rotated with no grace
- **THEN** a new token named "worker" with the same scopes and project is returned with its value
- **AND** the old token's value is refused as unauthenticated

#### Scenario: Rotating by name

- **WHEN** a token is rotated by its live name
- **THEN** the token carrying that name is the one rotated

### Requirement: A rotated token keeps its lifetime

The new token SHALL expire after the old token's lifetime counted from the rotation, and SHALL NOT expire when the old token did not.

#### Scenario: A 90 day token is good for another 90 days

- **GIVEN** a token issued with a 90 day expiry, 80 days ago
- **WHEN** it is rotated
- **THEN** the new token expires 90 days after the rotation

#### Scenario: A non-expiring token stays non-expiring

- **WHEN** a token without an expiry is rotated
- **THEN** the new token has no expiry

### Requirement: A rotation can leave the old token working for a grace period

A rotation SHALL accept a grace period from zero to 24 hours, zero by default, during which the old token SHALL still authenticate. The end of the window SHALL be stored on the old token and SHALL NOT depend on a background job. A grace outside the range SHALL be refused as invalid.

#### Scenario: The old token works inside the window

- **GIVEN** a token rotated with a grace of ten minutes
- **WHEN** the old value is presented five minutes later
- **THEN** it authenticates
- **AND** when it is presented eleven minutes after the rotation it is refused

#### Scenario: A grace over the cap is refused

- **WHEN** a rotation asks for a grace of 25 hours
- **THEN** it is refused as invalid naming the grace field

### Requirement: A retiring token can still be cut short

A token inside its grace window SHALL be listed as retiring, SHALL stop at once when revoked, SHALL stop at once when its user is disabled, and SHALL NOT be rotated again.

#### Scenario: Revoking a retiring token

- **GIVEN** a token inside its grace window
- **WHEN** it is revoked
- **THEN** its value is refused from that moment

#### Scenario: A retiring token is not rotated twice

- **WHEN** a token inside its grace window is rotated
- **THEN** the rotation is refused as a conflict

### Requirement: A token's own actor may rotate it

The actor a token acts as SHALL be allowed to rotate it without `token:admin`, so an agent can rotate its own credential. A project-restricted caller SHALL rotate only a token restricted to its own project. Rotating a token of another actor SHALL require `token:admin`.

#### Scenario: An agent rotates its own token

- **GIVEN** an agent token holding only task scopes
- **WHEN** the agent rotates that token
- **THEN** the rotation succeeds

#### Scenario: Another actor's token needs token:admin

- **GIVEN** a member without `token:admin`
- **WHEN** they rotate an agent's token
- **THEN** the rotation is refused naming `token:admin`

### Requirement: A rotated token never exceeds its rotator

A rotation SHALL be refused when the token carries a scope the rotator does not hold, applying the same `global:admin` and `account:read` rules as issuing a token.

#### Scenario: An administrator token without a scope cannot rotate a token carrying it

- **GIVEN** a token carrying `export` and a rotator holding `token:admin` but not `export`
- **WHEN** the rotator rotates it
- **THEN** the rotation is refused naming `export`

### Requirement: A rotation is audited without its secret

A rotation SHALL write one audit entry and one `token.rotated` event naming the old and new token identifiers, the actor the token acts as and when the old token stops. Rotating another actor's token SHALL be recorded under a different action from rotating your own. No token value SHALL appear in either.

#### Scenario: Rotating another actor's token

- **WHEN** an administrator rotates an agent's token
- **THEN** the trail records `token.rotate_other` naming both token identifiers and the agent
- **AND** neither value appears in the entry
