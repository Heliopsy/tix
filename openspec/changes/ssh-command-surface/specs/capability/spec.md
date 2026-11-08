## ADDED Requirements

### Requirement: SSH is a bound surface derived from the CLI

`ssh` SHALL be a capability surface every operation reaches unless exempted with a reason. An
operation SHALL bind SSH when it binds the CLI and carries no SSH exemption, and an operation the CLI
does not bind SHALL carry no SSH exemption, since SSH hosts the CLI.

#### Scenario: A new CLI-bound operation

- **WHEN** an operation gains a CLI binding
- **THEN** it is reachable over SSH, and the sshd parity test runs its command through the listener

#### Scenario: An exemption without a reason

- **WHEN** an SSH exemption carries an empty reason
- **THEN** the parity test fails

### Requirement: The sign-in surface of an SSH call is ssh

The sign-in surface a call is judged on SHALL be `ssh` for every call that arrived over the SSH
listener, whatever its source, and otherwise the surface its source maps to.

#### Scenario: Removing a factor over SSH

- **GIVEN** a tenant requiring a second factor for SSH only
- **WHEN** a person runs `ssh acme@host user totp rm`
- **THEN** the removal is refused as it is from a board session
