## ADDED Requirements

### Requirement: A spawned occurrence is announced like any created task

The next occurrence of a recurring task SHALL be announced by an ordinary `task.created` event, written in
the transaction that finished its predecessor, whose payload SHALL carry `recurs_from`, the predecessor's
reference. The predecessor's `task.transitioned` event SHALL carry `next`, the successor's reference.

#### Scenario: Subscriber sees both

- **WHEN** a recurring task is finished while a subscriber is watching
- **THEN** the subscriber receives the `task.transitioned` event naming `next` and a `task.created` event naming `recurs_from`
