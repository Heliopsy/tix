## ADDED Requirements

### Requirement: A generated signing secret is returned once

When a registration supplies no signing secret, the system SHALL generate one and return it in the registration's response as `secret`, on every transport including HTTP. A registration or update that supplies its secret, or an update that keeps the existing one, SHALL NOT return it. No listing, read, audit entry or event payload SHALL carry a signing secret.

#### Scenario: Registering over HTTP without a secret

- **WHEN** an administrator registers an endpoint over HTTP without a secret
- **THEN** the response carries the generated secret, and a later listing of that endpoint does not

#### Scenario: Supplying a secret

- **WHEN** an administrator registers an endpoint with their own secret
- **THEN** the response carries no secret

#### Scenario: History stays clean

- **WHEN** an endpoint is registered with a generated secret
- **THEN** no audit entry and no event payload contains that secret

### Requirement: Test ping to one endpoint

An administrator SHALL be able to ping one active endpoint. A ping SHALL record a `webhook.ping` event and an audit entry, and SHALL queue exactly one delivery of that event, to that endpoint only, whatever its event type filter. The response SHALL be the delivery, already attempted when the process drains inline.

#### Scenario: Ping an endpoint

- **WHEN** an administrator pings an active endpoint in the inline drain mode
- **THEN** the endpoint receives one signed delivery with event type webhook.ping and the delivery log lists it

#### Scenario: Other endpoints stay quiet

- **WHEN** an administrator pings one endpoint while another endpoint subscribes to every event
- **THEN** only the pinged endpoint has a delivery queued

#### Scenario: Inactive endpoint

- **WHEN** an administrator pings an inactive endpoint
- **THEN** the ping is refused as a precondition and nothing is queued

#### Scenario: Authorization

- **WHEN** a caller without the webhook admin scope pings an endpoint
- **THEN** the request is rejected as forbidden

### Requirement: A ping goes through the delivery queue

A ping SHALL be delivered by the same dispatcher, signature, egress policy and retry schedule as every other delivery, and SHALL be redeliverable like one. A process that queues no deliveries SHALL refuse a ping as a precondition.

#### Scenario: Redelivering a ping

- **WHEN** an administrator redelivers a ping's delivery
- **THEN** another attempt is queued carrying the same webhook.ping event

#### Scenario: Delivery turned off

- **WHEN** an administrator pings an endpoint in a process whose webhook delivery is off
- **THEN** the ping is refused as a precondition

### Requirement: The ping event type is in the vocabulary

`webhook.ping` SHALL be part of the closed event vocabulary, so an event stream subscription may name it.

#### Scenario: Subscribing to pings

- **WHEN** a client subscribes to the event stream with the type webhook.ping
- **THEN** the subscription is accepted and receives each ping's event
