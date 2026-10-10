# configuration Specification

## Purpose

Resolves every configuration key through one layered precedence chain, so no key is exempt and the resolved value is explainable.

## Requirements

### Requirement: Layered configuration precedence

Configuration SHALL be resolved from five layers applied uniformly to every configuration key, in descending precedence: command-line flags, environment variables, a `.env` file, the configuration file, and built-in defaults. A key set in a higher layer SHALL override the same key from every lower layer, and no key SHALL be exempt from the chain.

#### Scenario: Flag beats every other layer

- **WHEN** `database.dsn` is set in the configuration file, in a `.env` file, and in the environment, and `--db` is also passed on the command line
- **THEN** the value from `--db` is used

#### Scenario: Environment beats file layers

- **WHEN** `TIX_DATABASE_DSN` is exported and a different `database.dsn` is present in the configuration file
- **THEN** the exported environment value is used

#### Scenario: Defaults apply when nothing is set

- **WHEN** a key is absent from flags, environment, `.env`, and the configuration file
- **THEN** the built-in default for that key is used and the command succeeds

#### Scenario: Partial override leaves siblings intact

- **WHEN** only `output.format` is overridden by a flag
- **THEN** every other key retains the value it would have had from its own highest-priority layer

### Requirement: Generated environment variable for every key

Every configuration key SHALL have a corresponding environment variable whose name is derived mechanically from the key path by uppercasing it, replacing path separators with underscores, and prefixing `TIX_`. The product SHALL be fully configurable through environment variables alone, with no configuration file present.

#### Scenario: Key path maps to variable name

- **WHEN** the key `database.dsn` is read
- **THEN** the environment variable `TIX_DATABASE_DSN` supplies its value

#### Scenario: Nested key maps to variable name

- **WHEN** the key `hooks.mode` is read
- **THEN** the environment variable `TIX_HOOKS_MODE` supplies its value

#### Scenario: Full operation with no configuration file

- **WHEN** no configuration file exists anywhere on the system and every required setting is exported as a `TIX_*` variable
- **THEN** commands run with those settings and no error about a missing configuration file is produced

#### Scenario: Every key is reachable from the environment

- **WHEN** the set of configuration keys is enumerated
- **THEN** each key has exactly one generated `TIX_*` variable name and no two keys generate the same name

### Requirement: Dotenv discovery without clobbering the real environment

tix SHALL look for a `.env` file by walking up from the working directory and SHALL load the first one it finds. A variable already present in the process environment SHALL NOT be overwritten by a value in `.env`.

#### Scenario: .env found in a parent directory

- **WHEN** the working directory has no `.env` but a parent directory does
- **THEN** the parent's `.env` is loaded

#### Scenario: Exported variable wins over .env

- **WHEN** `TIX_DATABASE_DSN` is exported in the shell and `.env` also defines `TIX_DATABASE_DSN`
- **THEN** the exported value is used and the `.env` value is ignored

#### Scenario: Nearest .env wins

- **WHEN** both the working directory and a parent contain a `.env`
- **THEN** only the one in the working directory is loaded

#### Scenario: No .env present

- **WHEN** no `.env` exists between the working directory and the filesystem root
- **THEN** resolution continues with the remaining layers and no error is reported

### Requirement: Configuration file location resolution

The configuration file SHALL be located by checking `$TIX_CONFIG`, then `$XDG_CONFIG_HOME/tix/config.yaml`, then `~/.config/tix/config.yaml`, using the first path that exists. A missing configuration file SHALL NOT be an error.

#### Scenario: Explicit path takes priority

- **WHEN** `TIX_CONFIG` points at an existing file and `~/.config/tix/config.yaml` also exists
- **THEN** the file named by `TIX_CONFIG` is used

#### Scenario: XDG path used when set

- **WHEN** `TIX_CONFIG` is unset and `XDG_CONFIG_HOME` is set to a directory containing `tix/config.yaml`
- **THEN** that file is used

#### Scenario: Missing file is not an error

- **WHEN** none of the candidate paths exist
- **THEN** the command runs using environment, `.env`, and defaults, and exits successfully

#### Scenario: Explicit path that does not exist

- **WHEN** `TIX_CONFIG` names a path that does not exist
- **THEN** the command fails with a usage error naming the missing path

### Requirement: Named contexts

A context SHALL be a named bundle of connection and identity settings comprising either a database DSN or a server URL, plus an optional token, tenant, default project, authentication mode, and hook mode. A context SHALL NOT carry both a database DSN and a server URL.

#### Scenario: Context defines a remote target

- **WHEN** a context sets a server URL and a token
- **THEN** operations using that context are performed against that server with that token

#### Scenario: Context defines a local target

- **WHEN** a context sets a database DSN
- **THEN** operations using that context are performed directly against that database

#### Scenario: Both endpoints in one context is rejected

- **WHEN** a context is defined with both a database DSN and a server URL
- **THEN** the configuration is rejected with a usage error identifying the offending context

#### Scenario: Default project supplied by context

- **WHEN** the active context names a default project and a command that takes a project is run without one
- **THEN** the context's default project is used

### Requirement: Context management commands

tix SHALL provide `tix ctx list`, `tix ctx use`, `tix ctx show`, `tix ctx add`, and `tix ctx rm` for managing contexts, and these SHALL persist changes to the configuration file.

#### Scenario: Listing contexts

- **WHEN** `tix ctx list` is run
- **THEN** every defined context is listed and the current one is marked

#### Scenario: Switching the current context

- **WHEN** `tix ctx use work` is run and `work` exists
- **THEN** `current_context` is persisted as `work` and subsequent commands use it

#### Scenario: Switching to an unknown context

- **WHEN** `tix ctx use nope` is run and no context named `nope` exists
- **THEN** the command fails with a not-found error and the current context is unchanged

#### Scenario: Removing the current context

- **WHEN** `tix ctx rm` removes the context that is currently selected
- **THEN** the context is deleted and `current_context` is cleared so resolution falls through to the remaining sources

#### Scenario: Secrets are not printed in full

- **WHEN** `tix ctx show` displays a context that carries a token
- **THEN** the token is redacted unless the caller explicitly requests the raw value

### Requirement: Per-invocation context override

A `--ctx <name>` flag SHALL select a context for a single invocation without changing the persisted current context.

#### Scenario: One-off context selection

- **WHEN** `tix --ctx staging task list` is run while the current context is `work`
- **THEN** the command uses `staging` and the persisted current context remains `work`

#### Scenario: Unknown context name

- **WHEN** `--ctx` names a context that is not defined
- **THEN** the command fails with a usage error and performs no operation

### Requirement: Per-directory context discovery

tix SHALL detect a project-local context by looking for `.tix.yaml` or `.tix/config.yaml`, walking up from the working directory and stopping at the git repository root. The candidate filenames SHALL be configurable.

#### Scenario: Local file found in the working directory

- **WHEN** the working directory contains `.tix.yaml` naming a context
- **THEN** that context is used for commands run from that directory

#### Scenario: Walk stops at the git root

- **WHEN** no local configuration file exists inside the repository but one exists in a directory above the git root
- **THEN** that file is not used

#### Scenario: Nearest file wins

- **WHEN** a subdirectory and the repository root both contain a local configuration file
- **THEN** the file in the subdirectory is used

#### Scenario: Custom filenames

- **WHEN** the discovery filenames are configured to a different set of names
- **THEN** only files matching the configured names are considered

### Requirement: Disabling discovery

Per-directory discovery SHALL be disabled when `discovery.enabled` is set to `false` or when `--no-discovery` is passed.

#### Scenario: Flag disables discovery

- **WHEN** `--no-discovery` is passed from a directory containing `.tix.yaml`
- **THEN** the local file is ignored and the context is resolved from the remaining sources

#### Scenario: Setting disables discovery

- **WHEN** `discovery.enabled` is `false` in the configuration file
- **THEN** no directory walk is performed for any command

#### Scenario: Discovery on by default

- **WHEN** neither the flag nor the setting is present
- **THEN** discovery is performed

### Requirement: Source-attributed configuration inspection

`tix config show --sources` SHALL print every effective configuration value together with the layer that supplied it.

#### Scenario: Attribution per value

- **WHEN** `tix config show --sources` is run
- **THEN** each key is printed with its effective value and an identifier of the originating layer, one of flag, environment, dotenv, file, or default

#### Scenario: Overridden value reports the winning layer

- **WHEN** a key is set in both the configuration file and the environment
- **THEN** that key is reported as coming from the environment

#### Scenario: Machine-readable attribution

- **WHEN** `tix config show --sources -o json` is run
- **THEN** the output is valid JSON in which every entry carries the key, the effective value, and the source layer

#### Scenario: Secrets are redacted

- **WHEN** the effective configuration includes a token or password
- **THEN** its value is redacted in the output while its source layer is still reported

### Requirement: First-run bootstrap

On first use against a database that does not yet exist or has never been initialized, tix SHALL create the database, apply all migrations, and create a default tenant together with a small set of starter projects, one of which SHALL be the default project, without requiring any prior command. The starter projects SHALL be seeded only when the tenant is created, never on a later run, so that a project deleted on purpose is not recreated and an existing installation gains nothing on upgrade. Each starter project SHALL carry a distinct colour and an icon so the lists are distinguishable on sight, and seeding beyond the default project SHALL be suppressible for an installation that wants none.

#### Scenario: Fresh machine

- **WHEN** a mutating command is run on a machine with no configuration file and no existing database
- **THEN** the database is created, migrations are applied, the default tenant and the starter projects are created, and the command completes successfully

#### Scenario: Bootstrap is idempotent

- **WHEN** a second command runs against the already-bootstrapped database
- **THEN** no duplicate tenant or project is created and no migration is reapplied

#### Scenario: A deleted project is not recreated

- **WHEN** a starter project is deleted and a later command bootstraps again
- **THEN** the deleted project stays deleted

#### Scenario: An existing installation is untouched

- **WHEN** a database bootstrapped before the starter projects existed is opened
- **THEN** no project is added to it

#### Scenario: A task still needs no project named

- **WHEN** a task is created without naming a project on an installation carrying the starter projects
- **THEN** it is created in the default project rather than refused as ambiguous

#### Scenario: Bootstrap failure is reported clearly

- **WHEN** the database path cannot be created because of permissions
- **THEN** the command fails with an error naming the path and the reason, and leaves no partially initialized database in use

### Requirement: Configuration validation

Invalid configuration SHALL be rejected with an error identifying the offending key and the layer it came from, and SHALL NOT be silently ignored or coerced.

#### Scenario: Unknown key in the configuration file

- **WHEN** the configuration file contains a key tix does not recognize
- **THEN** the command fails with an error naming the key and the file

#### Scenario: Wrong value type

- **WHEN** a key expecting a duration receives a value that is not a duration
- **THEN** the command fails with an error naming the key, the received value, and the expected form

#### Scenario: Invalid value from the environment

- **WHEN** `TIX_HOOKS_MODE` is set to a value outside the allowed set
- **THEN** the command fails with an error naming the variable and listing the allowed values

### Requirement: A flag nobody typed is not a layer

A command flag that also names a configuration key SHALL contribute to resolution only when the operator
typed it. A flag left unset SHALL leave the value the environment, the `.env` file, the configuration file or
the built-in default supplied, even when the flag declares a default of its own.

#### Scenario: The configuration file decides the serve address

- **WHEN** `server.listen` names an address in a configuration file and `tix serve` runs with no `--listen`
- **THEN** the server binds that address, and a request to it is answered

#### Scenario: A typed flag still wins

- **WHEN** `server.listen` names one address in a configuration file and `tix serve --listen` names another
- **THEN** the server binds the address the flag names

#### Scenario: The environment decides the address

- **WHEN** `TIX_SERVER_LISTEN` names an address and `tix serve` runs with no `--listen`
- **THEN** the server binds that address, so a container can be pointed at one without a command line

### Requirement: The configured token is the credential

The token a command authenticates with SHALL be resolved through the documented layers. `--token` and
`TIX_TOKEN` SHALL be its flag layer, and `server.token`, a `TIX_SERVER_TOKEN`, a `.env` entry and the token a
named context carries SHALL each be presented when no layer above them supplies one.

#### Scenario: A configuration file supplies the token

- **WHEN** a configuration file names a server URL and a `server.token`, and a command runs with no `--token`
  and no `TIX_TOKEN`
- **THEN** that token is presented and the command is authenticated

#### Scenario: A flag overrides the configured token

- **WHEN** a configuration file names a `server.token` and `--token` names another
- **THEN** the token the flag names is presented

### Requirement: Every configuration key has a named consumer

Every key `internal/config` defines SHALL be recorded either as read by a named consumer or as shadowed by a
named flag on a named command. A key in neither state SHALL fail the test suite rather than ship as a
setting that does nothing.

#### Scenario: A new key with nothing reading it

- **WHEN** a field is added to the configuration structure and nothing is recorded about which consumer
  reads it
- **THEN** the suite fails, naming the key and its generated environment variable

#### Scenario: A shadowed key loses its fallback

- **WHEN** a key that a flag shadows stops being copied from the resolved configuration
- **THEN** the suite fails, naming the key, the flag and the value the command used instead

### Requirement: The number of rows a listing shows is configured per surface

Each surface that renders a listing SHALL take its page size from its own configuration key: `cli.page_size`
for the command line and `web.page_size` for the browser. Both SHALL default to 25 rows, and both SHALL be
resolved through the documented layers, where a flag beats the environment, which beats a `.env` entry, which
beats a configuration file, which beats the built-in default.

#### Scenario: A configuration file decides how many rows a listing prints

- **WHEN** `cli.page_size` names a number in a configuration file and a listing command runs with no `--limit`
- **THEN** the listing prints that many rows, and reports that a further page is available

#### Scenario: The environment decides how many rows a listing prints

- **WHEN** `TIX_CLI_PAGE_SIZE` names a number and a listing command runs with no `--limit`
- **THEN** the listing prints that many rows

#### Scenario: A typed limit still wins

- **WHEN** `cli.page_size` names one number and `--limit` names another
- **THEN** the listing prints the number the flag names

#### Scenario: A limit inside a filter expression still wins

- **WHEN** `cli.page_size` names one number and `--filter` carries a `limit:` term naming another
- **THEN** the listing prints the number the expression names

#### Scenario: The browser shows the configured number of rows

- **WHEN** `web.page_size` names a number and a browser listing is opened
- **THEN** that page carries that many rows

#### Scenario: Nothing configured shows twenty-five rows

- **WHEN** no layer sets a page size and a browser listing is opened
- **THEN** that page carries 25 rows

### Requirement: A page size outside the contract's range is refused

A configured page size below 1 or above the maximum a listing may return SHALL be refused when configuration
is resolved, naming the key and the layer that supplied it. It SHALL NOT be clamped or replaced by a default.

#### Scenario: A page size of zero is refused

- **WHEN** a layer sets `cli.page_size` to `0`
- **THEN** the command fails, names `cli.page_size`, and prints no rows

#### Scenario: A negative page size is refused

- **WHEN** a layer sets `web.page_size` to a negative number
- **THEN** configuration is refused, naming `web.page_size`

#### Scenario: An absurd page size is refused

- **WHEN** a layer sets a page size above the maximum a listing may return
- **THEN** configuration is refused, naming that key

### Requirement: A reader can ask one browser page for a different size

A browser listing SHALL accept a `limit` query parameter naming how many rows that page carries, overriding
the configured size for that request. The listing's own paging controls SHALL carry the requested size, so a
walk keeps the size it was asked for. A `limit` that is not a usable number SHALL leave the configured size
in place rather than failing the screen.

#### Scenario: A request asks for more rows than the configured size

- **WHEN** a browser listing is opened with a `limit` naming a number inside the accepted range
- **THEN** that page carries that many rows

#### Scenario: Paging keeps the requested size

- **WHEN** a reader opens a listing with a `limit` and follows the next-page control
- **THEN** the following page carries the same number of rows

#### Scenario: An unusable requested size is ignored

- **WHEN** a browser listing is opened with a `limit` that is not a number, is zero, is negative, or is above
  the maximum a listing may return
- **THEN** the page renders with the configured size
