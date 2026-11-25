## ADDED Requirements

### Requirement: A shared server reaches no internal address for a sync source

On `tix serve` every request a sync source makes SHALL be held to the operator's egress policy, checked on
the address the connection is about to use and on every redirect hop. By default loopback, link-local,
private, unique-local and other special-purpose addresses SHALL be refused, and only `http` and `https`
urls accepted.

#### Scenario: A source pointing at the metadata endpoint

- **WHEN** a tenant admin on `tix serve` saves a jira source with `url: http://169.254.169.254/latest`
- **THEN** the save fails as invalid on `url` with "address not allowed"

#### Scenario: A name that resolves to loopback

- **GIVEN** a bound source whose url is a name resolving to `127.0.0.1`
- **WHEN** a run starts on `tix serve`
- **THEN** the run fails with "address not allowed" and the loopback server receives no request

#### Scenario: A redirect into the network

- **WHEN** an allowed source answers with a redirect to a loopback address
- **THEN** the hop is refused and the loopback address receives no request

### Requirement: A sync refusal discloses nothing about the network

A refused sync request SHALL say "address not allowed" and SHALL NOT name the address a host resolved to,
and SHALL NOT be retried.

#### Scenario: Probing an internal name

- **WHEN** a run is refused because its host resolves to a private address
- **THEN** the error names the system and the config key, and no address

### Requirement: A direct command reaches what the operator can

A command run against the database directly SHALL NOT hold sync sources to the egress policy, as the
operator's own process.

#### Scenario: An internal tracker from the operator's shell

- **WHEN** the operator runs `tix sync run` without a server against a source at `http://10.1.2.3`
- **THEN** the run reaches `10.1.2.3`
