## ADDED Requirements

### Requirement: Public SSH address

The configuration SHALL accept `ssh.public_address` (`TIX_SSH_PUBLIC_ADDRESS`), a host or `host:port`
naming where people reach the SSH listener. It SHALL change nothing about where the listener binds;
the browser interface SHALL use it in the SSH commands it shows.

#### Scenario: The address is shown, not bound

- **GIVEN** `ssh.public_address: tix.example.com:2020` and `tix serve --ssh-listen 0.0.0.0:2222`
- **WHEN** the server starts
- **THEN** the listener binds port 2222, and the connect screen shows port 2020
