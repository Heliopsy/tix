# ssh-access

## MODIFIED Requirements

### Requirement: Persisted host key

The listener SHALL load a host key from a configurable path, generating one on first run with permissions
readable only by its owner. On a key file readable by anyone else it SHALL narrow the mode back to 0600,
report that it did so, and continue; it SHALL refuse to start only when it cannot establish that mode.

#### Scenario: First run

- **WHEN** the listener starts and no host key exists at the configured path
- **THEN** a key is generated, written at mode 0600, and reused unchanged on every later start

#### Scenario: A loose host key

- **WHEN** the host key file is readable by anyone but its owner
- **THEN** the listener narrows it to 0600, logs a warning naming the mode it found, and starts with the
  same key

#### Scenario: A host key that cannot be narrowed

- **WHEN** the host key file is readable by anyone but its owner and the mode cannot be changed
- **THEN** the listener refuses to start and names the file and its mode
