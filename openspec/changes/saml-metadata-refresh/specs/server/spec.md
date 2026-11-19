## ADDED Requirements

### Requirement: The server rereads SAML metadata as a background worker

`tix serve` SHALL run a SAML metadata refresher beside its other background workers when the refresh interval is positive. It SHALL visit every live tenant, act as that tenant's system actor, and decide whether a provider is due from the time of the provider's last read, so a restart or a second replica does not read a provider early. A failed read SHALL be logged and SHALL NOT stop the worker.

#### Scenario: Every tenant

- **WHEN** two tenants each have a due SAML provider
- **THEN** one pass of the refresher reads both

#### Scenario: Another replica just read it

- **WHEN** a provider was read a minute ago by another process
- **THEN** this process does not read it until the interval has passed
