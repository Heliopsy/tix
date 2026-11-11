## ADDED Requirements

### Requirement: A reaction emits its own type and notifies nobody

Giving a reaction SHALL emit `comment.reacted` and taking it back SHALL emit `comment.unreacted`, each with
the comment as subject and the task's ref, the task, the comment and the reaction in the payload, and an audit
entry beside it. A change that writes nothing SHALL emit nothing. A reaction SHALL create no inbox item.

#### Scenario: A star reader is not told

- **GIVEN** Bob starred a task
- **WHEN** Ada reacts to a comment on it and later takes the reaction back
- **THEN** a comment reacted and a comment unreacted event are emitted and Bob's inbox is unchanged
