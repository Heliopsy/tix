## ADDED Requirements

### Requirement: Saved filters are owned by an actor

The store SHALL keep saved filters keyed by tenant and identifier, each with its owning actor, a name, a
filter expression, a view, a sort, a shared flag and the owner's position for it. A listing for an actor
SHALL return the filters that actor owns and every shared filter of the tenant, and nothing from another
tenant. Removing an actor or a tenant SHALL remove its filters. Saved filters SHALL not travel in
snapshots.

#### Scenario: A private filter stays private

- **GIVEN** alice saved a filter without sharing it
- **WHEN** bob in the same tenant lists saved filters
- **THEN** alice's filter is not among them

#### Scenario: A shared filter is listed for the tenant

- **GIVEN** alice saved a shared filter
- **WHEN** bob in the same tenant lists saved filters
- **THEN** alice's filter is among them, after bob's own
