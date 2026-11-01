## ADDED Requirements

### Requirement: Calendar feeds are stored by hash

The store SHALL keep calendar feeds in `calendar_feeds`, keyed by identifier and scoped to a tenant,
holding the owning actor, the SHA-256 hash of the secret, the filter, the creation instant and the
revocation instant. At most one unrevoked feed SHALL exist per tenant and actor. Removing an actor
or a tenant SHALL remove its feeds.

#### Scenario: One live feed per actor

- **WHEN** two unrevoked feeds are written for one actor
- **THEN** the second write fails
