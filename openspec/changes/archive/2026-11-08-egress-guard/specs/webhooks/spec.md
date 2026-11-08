## ADDED Requirements

### Requirement: Deliveries are held to the shared egress policy

Webhook endpoints and inbox channels SHALL be held to the operator's egress policy on every surface: when
saved, and at delivery on the address the connection is about to use. A delivery SHALL NOT follow a
redirect. `webhooks.allow_private_targets` SHALL open every internal address for them alone.

#### Scenario: A loopback receiver

- **WHEN** an endpoint or channel targets an httptest server on `127.0.0.1` and no allowance is set
- **THEN** the save is refused with "address not allowed", and a stored one is never reached at delivery

#### Scenario: The operator's allow list

- **WHEN** `server.egress_allow` lists `127.0.0.0/8`
- **THEN** a delivery to the loopback receiver succeeds

#### Scenario: A redirect into the network

- **WHEN** an allowed receiver answers a delivery with a redirect to a loopback address
- **THEN** the delivery is recorded failed with the redirect status and the loopback address receives nothing
