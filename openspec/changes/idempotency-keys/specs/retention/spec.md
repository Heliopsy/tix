## ADDED Requirements

### Requirement: Expired idempotency keys are pruned

Pruning SHALL delete the tenant's idempotency keys whose window has passed, at most the run's limit of them, selected in expiry order through an index without `OFFSET`, and SHALL report the number removed as `idempotency_keys`. A dry run SHALL count them and delete nothing. A key past its window SHALL be treated as absent before it is pruned.

#### Scenario: A key past its window

- **WHEN** a key was kept with a 24 hour window and pruning runs 25 hours later
- **THEN** the key is deleted and the result reports idempotency_keys 1

#### Scenario: A key inside its window

- **WHEN** pruning runs one hour after a key was kept with a 24 hour window
- **THEN** the key remains and a repeat of its request is still replayed
