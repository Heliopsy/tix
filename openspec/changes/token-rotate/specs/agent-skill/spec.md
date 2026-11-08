## ADDED Requirements

### Requirement: The skill tells an agent how to rotate its own token

The tix skill SHALL tell an agent how to rotate the token it runs on with `tix token rotate`, with a grace window so it can swap to the new value without downtime, and that the new value is printed once.

#### Scenario: An agent asked to rotate its credential

- **WHEN** an agent reads the skill's token guidance
- **THEN** it finds the rotate command, the grace flag and the advice to store the printed value before the window closes
