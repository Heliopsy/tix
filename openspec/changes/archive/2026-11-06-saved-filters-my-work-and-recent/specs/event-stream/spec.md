## ADDED Requirements

### Requirement: Saved filter events

The event vocabulary SHALL include `filter.saved`, `filter.updated` and `filter.deleted`, each with
subject type `filter` and a payload naming the filter and whether it is shared.

#### Scenario: Saving emits an event

- **WHEN** a reader saves a filter
- **THEN** one `filter.saved` event is emitted in the same transaction as the row and its audit entry
