## ADDED Requirements

### Requirement: The API gives and takes back reactions on a comment

`PUT /api/v1/comments/{id}/reactions/{reaction}` SHALL give the caller's reaction and `DELETE` on the same path
SHALL take it back, each answering 200 with the comment. The reaction SHALL be named or given as its emoji.
Both SHALL need `comment:write` on the task; an unknown reaction SHALL be 400 and a deleted comment 404.

#### Scenario: Toggling by name and by emoji

- **WHEN** a caller PUTs heart twice and then PUTs the rocket emoji
- **THEN** the answer carries heart and rocket, each with count 1 and `reacted: true`

#### Scenario: An unknown reaction

- **WHEN** a caller PUTs `party_parrot`
- **THEN** the answer is 400 and nothing is written

### Requirement: A comment read carries its reactions

Every comment returned to a caller SHALL carry `reactions`, one entry per reaction given in the fixed order,
each with `reaction`, `emoji`, `count`, `reacted` for the caller and `actor_ids`, earliest first. The field
SHALL be absent when nobody reacted.

#### Scenario: Reacted is the caller's

- **GIVEN** Ada and Bob both reacted with thumbs_up and only Ada with rocket
- **WHEN** Bob lists the task's comments
- **THEN** thumbs_up has count 2 and `reacted: true`, and rocket has count 1 and `reacted: false`
