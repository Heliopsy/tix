## ADDED Requirements

### Requirement: A flag nobody typed is not a layer

A command flag that also names a configuration key SHALL contribute to resolution only when the operator
typed it. A flag left unset SHALL leave the value the environment, the `.env` file, the configuration file or
the built-in default supplied, even when the flag declares a default of its own.

#### Scenario: The configuration file decides the serve address

- **WHEN** `server.listen` names an address in a configuration file and `tix serve` runs with no `--listen`
- **THEN** the server binds that address, and a request to it is answered

#### Scenario: A typed flag still wins

- **WHEN** `server.listen` names one address in a configuration file and `tix serve --listen` names another
- **THEN** the server binds the address the flag names

#### Scenario: The environment decides the address

- **WHEN** `TIX_SERVER_LISTEN` names an address and `tix serve` runs with no `--listen`
- **THEN** the server binds that address, so a container can be pointed at one without a command line

### Requirement: The configured token is the credential

The token a command authenticates with SHALL be resolved through the documented layers. `--token` and
`TIX_TOKEN` SHALL be its flag layer, and `server.token`, a `TIX_SERVER_TOKEN`, a `.env` entry and the token a
named context carries SHALL each be presented when no layer above them supplies one.

#### Scenario: A configuration file supplies the token

- **WHEN** a configuration file names a server URL and a `server.token`, and a command runs with no `--token`
  and no `TIX_TOKEN`
- **THEN** that token is presented and the command is authenticated

#### Scenario: A flag overrides the configured token

- **WHEN** a configuration file names a `server.token` and `--token` names another
- **THEN** the token the flag names is presented

### Requirement: Every configuration key has a named consumer

Every key `internal/config` defines SHALL be recorded either as read by a named consumer or as shadowed by a
named flag on a named command. A key in neither state SHALL fail the test suite rather than ship as a
setting that does nothing.

#### Scenario: A new key with nothing reading it

- **WHEN** a field is added to the configuration structure and nothing is recorded about which consumer
  reads it
- **THEN** the suite fails, naming the key and its generated environment variable

#### Scenario: A shadowed key loses its fallback

- **WHEN** a key that a flag shadows stops being copied from the resolved configuration
- **THEN** the suite fails, naming the key, the flag and the value the command used instead
