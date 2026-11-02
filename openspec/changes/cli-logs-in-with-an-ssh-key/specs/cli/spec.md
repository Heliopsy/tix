## ADDED Requirements

### Requirement: The command line logs in with an SSH key

`tix login --ssh-key` SHALL sign the server's challenge with each key in `ssh-agent`, then each default
key file under `~/.ssh`, until one is accepted, and print the session as `tix login` does. `tix login
--ssh-key PATH` SHALL use only that private key, asking for its passphrase on the terminal without echo
when it is encrypted. `--ssh-key` SHALL NOT be combined with `--email` or `--password`. The CLI SHALL
compute the namespace from the server address it was given and SHALL refuse to sign when the server
announces a different one. A second factor SHALL be asked for exactly as after a password.

#### Scenario: Logging in with a key file

- **WHEN** `tix login --ssh-key ./id_ed25519 -o json` runs against a server where that key is enrolled
- **THEN** it prints the session and exits 0

#### Scenario: Logging in with the agent

- **GIVEN** an agent holding an unenrolled key and an enrolled key
- **WHEN** `tix login --ssh-key` runs
- **THEN** the enrolled key is used and the session is printed

#### Scenario: No key is accepted

- **WHEN** none of the keys tried is enrolled
- **THEN** it exits 5 naming the fingerprints it tried

### Requirement: Logging in does not need a credential first

`tix login` SHALL reach a remote server without first confirming a credential, since it exists to obtain
one.

#### Scenario: A remote login with no token configured

- **WHEN** `tix --server URL login --ssh-key KEY` runs with no token
- **THEN** it signs in rather than failing on the missing credential
