## ADDED Requirements

### Requirement: Inbox items are kept per recipient

The store SHALL keep inbox items keyed by tenant and identifier, each holding its recipient, its
kind (`mention`, `assigned`, `comment` or `status`), the task, the actor who caused it, the comment
when there is one, the statuses of a status change, a short excerpt, when it was created and when it
was read. Removing a tenant, a recipient or a task SHALL remove its items. Muted kinds SHALL be kept
per tenant and actor. Neither SHALL travel in snapshots.

#### Scenario: Purging a task removes its items

- **GIVEN** an inbox item about a task
- **WHEN** the task is purged
- **THEN** no inbox item names it

#### Scenario: Recipients are apart

- **WHEN** two actors each receive an item
- **THEN** each actor's inbox lists only their own
