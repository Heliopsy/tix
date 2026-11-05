## MODIFIED Requirements

### Requirement: An SSH session may name the board it opens

The listener SHALL read the session's command as the name of a registered interface followed by
its arguments, or, when the first word is not a registered name and is the only word, as the key of
a project to open, so that a client can land on a board rather than on the project list. This is
the SSH equivalent of `--project` on the local interface, which has no other channel here because
the username already carries the tenant.

#### Scenario: A client names a board

- **WHEN** an SSH client connects with a public key, requests a pseudo-terminal
  and sends a project key as the command
- **THEN** the session opens on that project's board rather than the project list

#### Scenario: A client names nothing

- **WHEN** an SSH client connects without a command
- **THEN** a hosted session opens the start menu and a sandbox session opens its seeded board

#### Scenario: A client sends something that is not a project key

- **WHEN** the command is empty, longer than a project key may be, carries
  characters a project key may not carry, or is several words not led by a registered interface
- **THEN** the session is refused before the interface is built, with a message naming every
  registered interface

#### Scenario: A client reaches for a shell

- **WHEN** the command is more than one argument and its first is not a registered interface
- **THEN** the session is refused with a message saying the listener serves an
  interface rather than a shell, naming every registered interface

## ADDED Requirements

### Requirement: The board interface opens any project key over SSH

`board KEY` SHALL open any project key, including one spelled like a registered interface.

#### Scenario: A client names a board through the board interface

- **WHEN** the command is `board watch`
- **THEN** the session opens on the board of the project keyed `watch`

### Requirement: SSH interfaces come from one registry

The interfaces an SSH session can host SHALL be listed once. Every registered interface SHALL have
a command launcher on the listener, a row on the start menu and a usage line in the refusal
message, and the registered names SHALL be reserved as command words, compared without regard to
case.

#### Scenario: A registered interface without a launcher

- **WHEN** an interface is registered and the listener has no launcher for it
- **THEN** the parity test fails

#### Scenario: A project spelled like an interface

- **GIVEN** a project keyed `watch`
- **WHEN** the command is `watch`
- **THEN** the session streams events, and `board watch` is how the project is opened

### Requirement: Watch over SSH

The `watch` command SHALL stream the event lines `tix watch` prints, as the session's actor and in
its tenant, through the same `Subscribe` permission check. It SHALL take project keys as
arguments and the `--project`, `--type`, `--actor`, `--since`, `--limit` and `-o ndjson` options,
resolving each project through the project read check before the stream starts.

#### Scenario: A project the actor cannot read

- **WHEN** the command is `watch secret` and the actor cannot read project `secret`
- **THEN** the session is refused with the reason and no subscription is opened

### Requirement: Watch over SSH colours only a terminal and ends only when stopped

The `watch` command SHALL write no escape codes without a pseudo-terminal, and colour with one whose
terminal type takes colour. It SHALL end on ctrl-c from a pseudo-terminal, client disconnect, the
keepalive limit, an administrator ending the connection and server shutdown, and SHALL NOT be closed
for idleness.

#### Scenario: Piped watch

- **WHEN** a client runs `ssh tix@host watch | grep claimed` without a pseudo-terminal
- **THEN** each committed event arrives as one plain line ending in a newline

#### Scenario: Watch in a terminal

- **WHEN** a client runs `ssh -t tix@host watch`
- **THEN** lines are coloured and ctrl-c ends the session with exit status zero

#### Scenario: Shutdown

- **WHEN** the server drains past its deadline while a watch is streaming
- **THEN** the client is told the server is shutting down and the session ends
