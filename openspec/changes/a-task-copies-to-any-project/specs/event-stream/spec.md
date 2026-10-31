## ADDED Requirements

### Requirement: A copy announces itself as a created task

Each task a copy creates SHALL emit `task.created` whose payload carries `copied_from` with the
source's reference, so an existing subscriber sees a copy as the new task it is.

#### Scenario: Subscriber sees the copy

- **WHEN** a subscriber is watching and OPS-3 is copied to WEB
- **THEN** it receives `task.created` for the WEB task with `copied_from: OPS-3`
