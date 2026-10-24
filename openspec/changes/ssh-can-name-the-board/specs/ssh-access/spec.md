# ssh-access

## ADDED Requirements

### Requirement: An SSH session may name the board it opens

The listener SHALL read the session's command as the key of a project to open,
so that a client can land on a board rather than on the project list. This is
the SSH equivalent of `--project` on the local interface, which has no other
channel here because the username already carries the tenant.

The project SHALL be opened as the session's own actor, through the same
service and the same scopes the project list would have used, so naming a
project reaches nothing that choosing it from the list would not have reached.

#### Scenario: A client names a board

- **WHEN** an SSH client connects with a public key, requests a pseudo-terminal
  and sends a project key as the command
- **THEN** the session opens on that project's board rather than the project list

#### Scenario: A client names nothing

- **WHEN** an SSH client connects without a command
- **THEN** a hosted session opens the project list and a sandbox session opens
  its seeded board, as each did before

#### Scenario: A client names a project it cannot reach

- **WHEN** the command is a well formed project key that this actor cannot open
- **THEN** the session opens on the project list carrying the reason, rather
  than being refused

#### Scenario: A client sends something that is not a project key

- **WHEN** the command is empty, longer than a project key may be, or carries
  characters a project key may not carry
- **THEN** the session is refused before the interface is built, with a message
  saying the argument is a project key to open

#### Scenario: A client reaches for a shell

- **WHEN** the command is more than one argument
- **THEN** the session is refused with a message saying the listener serves an
  interface rather than a shell
