## MODIFIED Requirements

### Requirement: One schema and one migration path

Both engines SHALL share a single logical schema and a single ordered migration sequence. A
migration SHALL NOT exist for one engine only; engine-specific statements SHALL live within the
same numbered migration step.

The engines SHALL NOT store different instants for the same value. Where an engine's column type is
coarser than the clock, the coarser resolution SHALL be the resolution the value is authored at, so
that what is handed to an engine is what the row holds and what a caller is told. Any remaining
precision difference SHALL be documented in the engine package rather than left for a reader to
discover from a disagreement.

A migration SHALL run with whatever access its login role has and nothing more. On PostgreSQL that
places a migration outside the row-level security policies the same sequence creates, so a step that
mutates tenant-scoped rows is not guaranteed to see them. That exposure SHALL be documented in the
engine package, naming which shipped migrations it reaches and what it would cost, for as long as the
migration runner does not account for it.

#### Scenario: Same migration count

- **WHEN** migrations are applied to a fresh SQLite database and a fresh PostgreSQL database
- **THEN** both record the same migration identifiers in the same order

#### Scenario: Engine-specific statement

- **WHEN** a migration needs a statement that applies only to PostgreSQL, such as a policy or a partition
- **THEN** it is part of that same migration step and is skipped on SQLite without changing the recorded sequence

#### Scenario: Equivalent entity set

- **WHEN** the same service operations run against each engine
- **THEN** they produce equivalent stored data and identical results apart from documented search differences

#### Scenario: An instant written to either engine comes back unchanged

- **WHEN** an instant authored at the resolution the coarser engine keeps is stored and read back
- **THEN** both engines return the instant that was written

#### Scenario: The migration runner's exposure to row-level security is written down

- **WHEN** a reader looks for whether a data migration can be trusted to see the rows it names
- **THEN** the PostgreSQL engine package states that a migration runs outside its own policies, which
  roles that affects, and which shipped migrations it reaches
