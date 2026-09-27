## ADDED Requirements

### Requirement: Only a resolved identity occupies listener capacity

The SSH listener SHALL NOT let a key it has not resolved to an actor occupy a share of the listener-wide
session cap. A session slot SHALL be taken only after the presented fingerprint has been resolved to the
actor the session runs as.

The capacity refusal SHALL be decided before the fingerprint is looked up, so that a listener with no slot
left refuses every key at the same point in the exchange, with the same message, having done the same work.

#### Scenario: A stranger mid-lookup does not hold the last slot

- **WHEN** a key that is not enrolled has authenticated and is still being looked up on a listener whose
  session cap is one, and an enrolled key then opens a session
- **THEN** the enrolled session is served, and the unenrolled key is refused holding no slot

#### Scenario: A full listener answers an enrolled and an unenrolled key alike

- **WHEN** the listener is at its session cap and a key opens a session
- **THEN** it is refused for capacity before the fingerprint is looked up, whether or not that fingerprint
  is enrolled

#### Scenario: The cap still refuses an enrolled key it cannot serve

- **WHEN** the listener is at its session cap because enrolled sessions are holding every slot, and another
  enrolled key opens a session
- **THEN** that session is refused with the capacity message

#### Scenario: A demo key is refused nothing between the two steps

- **WHEN** the listener runs in demo mode, where every key resolves to a sandbox of its own
- **THEN** a key that passed the capacity check is given a session slot
