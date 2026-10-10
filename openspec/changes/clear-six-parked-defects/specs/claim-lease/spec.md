## ADDED Requirements

### Requirement: A lease expiry is one instant, whatever the engine

A lease expiry SHALL be materialized at the coarsest resolution any supported storage engine
preserves, before it is written to a row or reported to a caller.

The expiry a claim or a renewal reports SHALL equal the expiry the task it returns carries, and SHALL
equal the expiry stored, on every engine.

Materialization SHALL truncate and SHALL NOT round. An expiry moved later than the instant it was
computed for would outlive what its holder was told, which is the same defect in the other direction.

#### Scenario: The reported expiry is the stored expiry

- **WHEN** a task is claimed, or its lease renewed, at an instant finer than the stored resolution
- **THEN** the expiry reported with the claim equals the expiry on the task returned with it
- **AND** both equal what the row holds

#### Scenario: The two engines agree about one claim

- **GIVEN** the same claim taken at the same instant against each supported engine
- **WHEN** the stored expiry is read back from each
- **THEN** both engines report the same instant

#### Scenario: Materialization never extends a lease

- **WHEN** an expiry is materialized from an instant that the stored resolution cannot represent
- **THEN** the materialized expiry is at or before that instant, never after it
