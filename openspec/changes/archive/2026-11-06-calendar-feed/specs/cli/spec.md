## ADDED Requirements

### Requirement: Calendar feed commands

The CLI SHALL provide `tix calendar show`, `tix calendar create [--filter EXPR]` (alias `url`),
`tix calendar rotate`, `tix calendar filter [EXPR]`, where no expression restores the default, and
`tix calendar revoke`. `create` and `rotate` SHALL print the feed URL, absolute when connected to a
server.

#### Scenario: Create a feed from the command line

- **WHEN** an operator connected to a server runs `tix calendar create --filter 'tag:ops'`
- **THEN** the command prints a URL that answers the feed without a credential

#### Scenario: Show never prints the secret

- **WHEN** an operator runs `tix calendar show -o json`
- **THEN** the output holds the filter and creation time and no secret
