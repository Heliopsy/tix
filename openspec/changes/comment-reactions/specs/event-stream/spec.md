## ADDED Requirements

### Requirement: Comment reaction events

The event vocabulary SHALL include `comment.reacted` and `comment.unreacted`, emitted when an actor adds or
removes an emoji reaction, each carrying the task's ref and id, the emoji and the acting actor. A call that
changed nothing SHALL emit neither.

#### Scenario: React and take back

- **WHEN** an actor reacts to a comment and then removes the reaction
- **THEN** a `comment.reacted` event and then a `comment.unreacted` event are emitted, each naming the emoji
