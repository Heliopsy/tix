## ADDED Requirements

### Requirement: The registry records the account screen's bindings

The capability registry SHALL bind `token.create`, `token.list`, `token.revoke`, `sshkey.enrol`, `sshkey.list`, `sshkey.revoke`, `totp.status`, `totp.enrol`, `totp.confirm`, `totp.codes` and `totp.remove` to the terminal account view, and SHALL no longer record a terminal gap against any of them.

#### Scenario: None of the eleven records a terminal gap

- **WHEN** the registry is asked for its terminal gaps
- **THEN** none of the eleven operations is among them
- **AND** the recorded terminal gap count is lower by eleven

### Requirement: The account log reaches the terminal

`account.log` SHALL be bound to the terminal account view rather than exempted, because the service decides who may read it and the browser reads it from any tenant session.

#### Scenario: The exemption is gone

- **WHEN** `account.log` is read from the registry
- **THEN** it declares a terminal binding and no terminal exemption

### Requirement: The terminal credential listings record that they list the reader's own

`token.list` and `sshkey.list` SHALL record a terminal limitation stating that the account view lists the signed-in actor's own, where the command line and the API take another actor.

#### Scenario: The partial binding says how far it reaches

- **WHEN** `token.list` is read from the registry
- **THEN** it declares a terminal binding and a terminal limitation
