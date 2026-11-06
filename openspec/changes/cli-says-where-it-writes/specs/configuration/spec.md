## MODIFIED Requirements

### Requirement: First-run bootstrap

On first use against a database that does not yet exist or has never been initialized, tix SHALL create the database, apply all migrations, and create a default tenant together with a small set of starter projects, one of which SHALL be the default project, without requiring any prior command, once the database has been asked for.

#### Scenario: Fresh machine

- **WHEN** a mutating command is run on a machine with no existing database after `tix init`, or with `target: local`
- **THEN** the database is created, migrations are applied, the default tenant and the starter projects are created, and the command completes successfully

#### Scenario: Bootstrap is idempotent

- **WHEN** a second command runs against the already-bootstrapped database
- **THEN** no duplicate tenant or project is created and no migration is reapplied

#### Scenario: A task still needs no project named

- **WHEN** a task is created without naming a project on an installation carrying the starter projects
- **THEN** it is created in the default project rather than refused as ambiguous

#### Scenario: Bootstrap failure is reported clearly

- **WHEN** the database path cannot be created because of permissions
- **THEN** the command fails with an error naming the path and the reason, and leaves no partially initialized database in use

### Requirement: The configured token is the credential

The token a command authenticates with SHALL be resolved through the documented layers. `--token` and
`TIX_TOKEN` SHALL be its flag layer, and `server.token`, a `TIX_SERVER_TOKEN`, a `.env` entry and the token a
named context carries SHALL each be presented when no layer above them supplies one, by every command.

#### Scenario: A configuration file supplies the token

- **WHEN** a configuration file names a server URL and a `server.token`, and a command runs with no `--token`
  and no `TIX_TOKEN`
- **THEN** that token is presented and the command is authenticated

#### Scenario: A flag overrides the configured token

- **WHEN** a configuration file names a `server.token` and `--token` names another
- **THEN** the token the flag names is presented

#### Scenario: A context's token reaches a command that dials first

- **WHEN** `tix ctx add x --server URL --token T --use` has run and `tix task show REF`, `tix claim next` or
  `tix status` runs with nothing exported
- **THEN** the request carries `Authorization: Bearer T`

## ADDED Requirements

### Requirement: Nothing configured is not a request for a local database

When no flag, environment variable, context or configuration names a server or a database, the target key is
not `local`, and no database exists at the default path, a command SHALL NOT create one. Without a terminal it
SHALL exit 6 naming `tix ctx add` and `tix init`; on a terminal it SHALL ask.

#### Scenario: An agent's first command

- **WHEN** `tix task add x` runs with no configuration and no terminal
- **THEN** it exits 6, the message names `tix ctx add NAME --server URL --token TOKEN --use` and `tix init`,
  and no database file exists afterwards

#### Scenario: A person at a terminal chooses the local database

- **WHEN** the same command runs on a terminal and the answer is `1`
- **THEN** the database is created and the command completes

### Requirement: Existing local use counts as opting in

A database already at the default path, `target: local` (`TIX_TARGET=local`), `--db`, `TIX_DATABASE_DSN` and a
context naming a database SHALL each be enough for a command to open the local database as before.
`tix init` SHALL write `target: local` and create the database.

#### Scenario: An install from before this change

- **WHEN** a database exists at the default path and nothing else is configured
- **THEN** commands use it without asking

### Requirement: The implicit local database is named on every use

While a command uses the default database because nothing named a target, it SHALL print one line on standard
error naming the path and how to connect a server. `--quiet` SHALL NOT hide it, it SHALL NOT appear on standard
output, and `cli.target_notice: false` SHALL drop it.

#### Scenario: JSON output stays clean

- **WHEN** `tix task ls -o json -q` runs against the default database
- **THEN** standard output parses as JSON and standard error carries the line naming the database
