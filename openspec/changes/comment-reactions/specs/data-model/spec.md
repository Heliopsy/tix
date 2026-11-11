## ADDED Requirements

### Requirement: A comment carries reactions from a fixed set

The system SHALL store reactions on comments from the fixed set thumbs_up, thumbs_down, laugh, hooray,
confused, heart, rocket and eyes, at most one of each per actor per comment, inside the comment's tenant.
Removing a comment for good or an actor SHALL remove their reactions.

#### Scenario: One per actor and reaction

- **GIVEN** Ada reacted to a comment with heart
- **WHEN** Ada reacts with heart again
- **THEN** the comment still carries one heart from Ada and nothing is written

#### Scenario: Another tenant's row stays out

- **GIVEN** a reaction row written under another tenant for a comment of this one
- **WHEN** this tenant reads the comment's reactions
- **THEN** the row is not among them
