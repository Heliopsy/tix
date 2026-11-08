## ADDED Requirements

### Requirement: An SSH session may run a tix command

When the first word of a session's command is the first word of a command the listener hosts, the
listener SHALL run that tix CLI command in-process, as the session's actor and in its tenant, against
the server's own service. It SHALL NOT start a shell or execute a program.

#### Scenario: Listing tasks over SSH

- **GIVEN** a key enrolled in tenant `acme`
- **WHEN** the client runs `ssh acme@host task ls -o json`
- **THEN** the session prints the tenant's tasks as JSON, as `tix task ls -o json` would, and exits 0

#### Scenario: Adding a task over SSH

- **WHEN** the client runs `ssh acme@host task add "x" -p INFRA`
- **THEN** the task is created in project INFRA by the key's actor

### Requirement: The hosted commands come from the capability registry

The commands an SSH session may run SHALL be exactly the CLI bindings of the capability registry
whose operation carries no SSH exemption, and the groups above them. Every other command SHALL be
refused with exit code 2 before anything runs.

#### Scenario: A command with no registry binding

- **WHEN** the client runs `ssh acme@host config ls` or `ssh acme@host serve`
- **THEN** the session exits 2 saying the command is not available over SSH

#### Scenario: An exempted command

- **WHEN** the client runs `ssh acme@host login`
- **THEN** the session exits 2 and nothing runs

### Requirement: Local target and credential flags are refused over SSH

A hosted command SHALL refuse `--db`, `--server`, `--token`, `--ctx`, `--config`, `--tenant`,
`--no-discovery`, `--allow-network-fs` and every `--log-*` flag with exit code 2, judged after the
command line is parsed.

#### Scenario: Pointing a command at another database

- **WHEN** the client runs `ssh acme@host task ls --db /var/lib/tix/other.db`
- **THEN** the session exits 2 naming the flag, and no database is opened

#### Scenario: Naming another tenant

- **WHEN** the client runs `ssh acme@host task ls --tenant other`
- **THEN** the session exits 2 and nothing of tenant `other` is read

### Requirement: A hosted command opens no file on the server

A hosted command SHALL refuse every flag or argument naming a file path other than `-`, with exit
code 2 and a message pointing at standard input or output, before it reads or writes anything.

#### Scenario: Writing an export to a server path

- **WHEN** the client runs `ssh acme@host export tasks -p INFRA --out /tmp/x.csv`
- **THEN** the session exits 2 and no file is created on the server

#### Scenario: Reading a mapping from a server path

- **WHEN** the client runs `ssh acme@host import tasks -p INFRA --mapping /etc/passwd`
- **THEN** the session exits 2 and the file is not read

### Requirement: Standard streams and the exit code pass through

A hosted command SHALL read the session's standard input, write its standard output and standard
error to the session's two streams, and end the session with the exit code the command line would
have returned.

#### Scenario: Export and import through redirection

- **WHEN** the client runs `ssh acme@host export tasks -p INFRA > infra.csv` and then
  `ssh acme@host import tasks -p OPS < infra.csv`
- **THEN** the file holds the CSV export and the tasks are imported into OPS

#### Scenario: A failing command

- **WHEN** the client runs `ssh acme@host task show NOPE-1`
- **THEN** the error is written to standard error and the session exits 3

### Requirement: Hosted commands are audited as SSH

Every call a hosted command makes SHALL carry the `ssh` access channel with the key fingerprint and
the client address, so an audit entry it writes records `via=ssh`.

#### Scenario: A task created over SSH

- **WHEN** the client runs `ssh acme@host task add "x"`
- **THEN** the audit entry for the creation records via `ssh` and the key's fingerprint

### Requirement: Hosted commands owe the second factor a board session owes

A command session SHALL owe a second factor exactly when a board session with the same key would,
and SHALL prove it before the command is read: at the board's prompt with a terminal, or once from a
`TIX_CODE` environment variable without one. Calls over SSH SHALL be judged on the `ssh` surface.

#### Scenario: A command without a terminal and without a code

- **GIVEN** a tenant requiring a second factor for SSH and an enrolled person
- **WHEN** the client runs `ssh acme@host task ls` with no terminal and no `TIX_CODE`
- **THEN** the session is refused naming `ssh -t` and `TIX_CODE`, and nothing is read

#### Scenario: A command with a valid code

- **WHEN** the same client sends a current code as `TIX_CODE`
- **THEN** the command runs

### Requirement: Hosted commands are bounded

A hosted command SHALL hold a session slot, register as a live connection, refuse more than 128
words or a word carrying a control character, read at most `server.max_import_bytes` of standard
input and be cancelled after the listener's command timeout.

#### Scenario: An oversized command line

- **WHEN** the command carries a word with an escape character
- **THEN** the session exits 2 before anything runs

#### Scenario: Input past the limit

- **WHEN** standard input carries more than `server.max_import_bytes`
- **THEN** the command fails naming the limit and nothing is imported

### Requirement: A demo listener hosts no commands

A listener in demo mode SHALL refuse every hosted command, because a sandbox key is a stranger.

#### Scenario: A sandbox visitor runs a command

- **WHEN** a client of a demo listener runs `ssh host task ls`
- **THEN** the session is refused and nothing runs

### Requirement: A hosted command whose client stops reading is stopped

A hosted command SHALL be stopped and its session closed when a write of its output has waited on the
client for longer than the listener's write stall timeout, two minutes by default.

#### Scenario: A client that stopped reading an export

- **WHEN** a client runs `ssh acme@host export` and reads nothing
- **THEN** the session is closed after the write stall timeout and its slot is given back

### Requirement: A running hosted command is stopped when its key loses standing

The listener SHALL resolve a running command's key again at its recheck interval, 30 seconds by
default, and SHALL stop the command with exit code 5 when the key no longer resolves to the same
actor, tenant and role.

#### Scenario: A key revoked mid-command

- **GIVEN** a hosted command waiting on standard input
- **WHEN** the key is revoked
- **THEN** the command stops with exit code 5 within one recheck interval

### Requirement: Hosted standard input and output are byte-exact

A hosted command's standard input and output SHALL carry the session's bytes unchanged with or
without a pseudo-terminal, and a prompt SHALL still see a terminal when one was requested.

#### Scenario: An export under ssh -t

- **WHEN** a client runs `ssh -t acme@host export tasks -p INFRA`
- **THEN** the output is byte for byte the output without `-t`

### Requirement: Each channel keeps its own state

The listener SHALL keep what one session channel owns, its actor, registration and watchdogs, out of
the connection's shared context, and SHALL end a channel's watchdogs when the channel ends.

#### Scenario: Two channels on one connection

- **WHEN** one connection runs a long command and a short one at once
- **THEN** the short one's end leaves the long one registered, and the long one's end leaves none

## MODIFIED Requirements

### Requirement: An SSH session may name the board it opens

The listener SHALL read the session's command as a registered interface and its arguments, as a
hosted command, or, when the only word is neither, as the key of a project to open, so that a client
can land on a board rather than on the project list. This is the SSH equivalent of `--project` on the
local interface.

#### Scenario: A client names a board

- **WHEN** an SSH client connects with a public key, requests a pseudo-terminal
  and sends a project key as the command
- **THEN** the session opens on that project's board rather than the project list

#### Scenario: A client names nothing

- **WHEN** an SSH client connects without a command
- **THEN** a hosted session opens the start menu and a sandbox session opens its seeded board

#### Scenario: A client sends something that is not a project key

- **WHEN** the command is longer than a project key may be, carries characters a project key may not
  carry, or is several words led by neither a registered interface nor a hosted command
- **THEN** the session is refused before the interface is built, with a message naming every
  registered interface and saying commands such as `task ls` run too

#### Scenario: A client reaches for a shell

- **WHEN** the command is `sh -c id` or `bash`
- **THEN** the session is refused with a message saying the listener serves tix rather than a shell
