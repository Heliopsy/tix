## ADDED Requirements

### Requirement: Tenant endpoints see inbox.updated

Tenant webhook endpoints SHALL stay tenant configuration filtered by event type and SHALL NOT receive
inbox items themselves, which go to personal channels. An endpoint subscribed to `inbox.updated` SHALL
receive it like any other event, recipients included.

#### Scenario: A bridge subscribes to inbox.updated

- **WHEN** an endpoint subscribed to `inbox.updated` exists and a comment notifies bob
- **THEN** the endpoint receives one delivery naming bob among the recipients

### Requirement: Personal notification channels

An actor SHALL be able to keep notification channels of their own, each with a name, a URL, a format
(`generic` signed JSON, `slack`, `discord` or `ntfy`), the inbox kinds it receives and an enabled flag.
Creating an inbox item SHALL queue, in the same transaction, one delivery to each enabled channel of the
item's recipient that receives the item's kind, and to no other actor's channel. The webhook dispatcher
SHALL drain these deliveries with the same claim, retry schedule, network guard and signature as tenant
webhook deliveries. A channel's signing secret SHALL be returned only by its creation. A test message and
a redelivery SHALL go through the same queue. Another actor's channel or delivery SHALL be not found. The
audit log SHALL NOT keep a channel's URL beyond its scheme and host.

#### Scenario: A mention reaches a Slack channel

- **GIVEN** bob has an enabled Slack channel receiving mentions
- **WHEN** ada mentions @bob in a comment on infra-42
- **THEN** the channel is posted a Slack message saying ada mentioned bob on infra-42, signed with its secret

#### Scenario: A kind the channel does not take

- **GIVEN** bob's channel receives only mentions
- **WHEN** a task is assigned to bob
- **THEN** nothing is queued for that channel

#### Scenario: Somebody else's item

- **WHEN** a comment notifies bob and carol
- **THEN** carol's channel is queued carol's item only
