## ADDED Requirements

### Requirement: Inbox items are not delivered per user

Webhook endpoints SHALL stay tenant configuration filtered by event type. An endpoint subscribed to
`inbox.updated` SHALL receive it like any other event, recipients included; there SHALL be no
per-user endpoint.

#### Scenario: A bridge subscribes to inbox.updated

- **WHEN** an endpoint subscribed to `inbox.updated` exists and a comment notifies bob
- **THEN** the endpoint receives one delivery naming bob among the recipients
