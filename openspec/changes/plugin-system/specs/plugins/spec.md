## ADDED Requirements

### Requirement: A plugin is an installed executable

A plugin SHALL be an executable file that tix runs as a child process. tix SHALL NOT require a plugin to
be written in any particular language, to link against any tix library, or to have been built with any
particular toolchain. tix SHALL NOT load a plugin into its own process.

#### Scenario: A shell script is a plugin

- **WHEN** an executable shell script is installed as a plugin and its name is invoked
- **THEN** it runs and its output reaches the caller

#### Scenario: A compiled program is a plugin

- **WHEN** an executable built from any language is installed as a plugin and its name is invoked
- **THEN** it runs without tix having been rebuilt or having loaded any shared object

#### Scenario: A file that is not executable is refused

- **WHEN** installation is attempted on a file without the execute bit
- **THEN** it is refused, naming the file, and nothing is added to the manifest

### Requirement: Plugins are installed explicitly and never discovered on PATH

tix SHALL NOT scan `$PATH`, the working directory, or any directory it does not own for candidate
plugins. A plugin SHALL run only when an entry naming it exists in the plugin manifest. An executable
placed into the plugin directory without being installed SHALL NOT be runnable as a plugin.

#### Scenario: An executable on PATH is not a plugin

- **WHEN** an executable named `tix-deploy` is on `$PATH` and `tix deploy` is invoked with nothing
  installed
- **THEN** tix reports an unknown command and does not execute it

#### Scenario: A file dropped into the plugin directory is not a plugin

- **WHEN** an executable is copied into the plugin directory by hand and its name is invoked
- **THEN** tix reports an unknown command, because the manifest has no entry for it

#### Scenario: Installation is what makes a plugin runnable

- **WHEN** the same executable is installed with `tix plugin install`
- **THEN** invoking its name runs it

### Requirement: The manifest records what is installed

tix SHALL keep a manifest of installed plugins under its own data directory. Each entry SHALL record the
plugin's name, the executable it resolves to, a cryptographic digest of that executable, where it was
installed from, when it was installed, and the scopes of any token granted to it. The manifest SHALL be
written only by the install and remove operations.

#### Scenario: An install is recorded

- **WHEN** a plugin is installed
- **THEN** the manifest gains an entry carrying its name, digest, source, install time and granted
  scopes

#### Scenario: A removal is recorded

- **WHEN** a plugin is removed
- **THEN** its manifest entry is gone and its executable is deleted from the plugin directory

#### Scenario: A corrupt manifest does not break tix

- **WHEN** the manifest cannot be parsed
- **THEN** built-in commands continue to work, plugin dispatch is refused with an error naming the
  manifest, and the manifest is not rewritten

### Requirement: A plugin's integrity and permissions are checked before every run

Before executing a plugin tix SHALL verify that the executable's digest matches the one recorded at
install time, that the executable and the plugin directory are owned by the user running tix, and that
neither is writable by group or other. A failed check SHALL refuse the run and SHALL say which check
failed.

#### Scenario: A replaced executable is refused

- **WHEN** a plugin's executable is modified after installation and its name is invoked
- **THEN** the run is refused with an error naming the plugin and the digest mismatch

#### Scenario: A world-writable plugin is refused

- **WHEN** the plugin directory or a plugin executable is writable by other users
- **THEN** the run is refused with an error naming the permission problem

#### Scenario: An unchanged plugin runs

- **WHEN** the digest matches and the permissions are safe
- **THEN** the plugin runs

### Requirement: Managing installed plugins

tix SHALL offer commands to install a plugin from a local path, list what is installed, show one
plugin's detail, and remove one. Listing SHALL name each plugin, its version if it reports one, its
source, and the scopes it holds. Every one of these SHALL answer `-o json`.

#### Scenario: Listing what is installed

- **WHEN** plugins are installed and the list is requested
- **THEN** each is named with its source and its granted scopes, and nothing that is not installed
  appears

#### Scenario: Listing with nothing installed

- **WHEN** no plugin is installed and the list is requested
- **THEN** an empty list is returned rather than an error

#### Scenario: Detail on one plugin

- **WHEN** detail is requested for an installed plugin
- **THEN** its name, executable, digest, source, install time, granted scopes and reachability are
  reported

#### Scenario: Removing a plugin that is not installed

- **WHEN** removal names a plugin with no manifest entry
- **THEN** it is refused as not found

### Requirement: Installation states what the plugin is trusted with

Installation SHALL tell the operator, before anything is written, that the plugin will run as an
ordinary process with their own filesystem and network access, and SHALL name the scopes of any token
being granted. Installation SHALL require confirmation unless the caller explicitly assumes it.

#### Scenario: The trust decision is shown

- **WHEN** an install is started interactively
- **THEN** the operator is told the plugin runs with their own access, is shown the scopes requested,
  and nothing is written until they confirm

#### Scenario: Declining writes nothing

- **WHEN** the operator declines at the confirmation
- **THEN** no executable is copied, no manifest entry is written, and no token is minted

### Requirement: A plugin never inherits the caller's credential

tix SHALL remove every tix credential variable from the environment handed to a plugin, whether or not
it was set in the caller's own environment. A plugin SHALL receive a credential only when a token was
granted to it at install time, and that token SHALL carry only the scopes recorded in its manifest
entry.

#### Scenario: The caller's token does not reach the plugin

- **WHEN** the caller has a token in their environment and runs a plugin with no grant
- **THEN** the plugin's environment carries no tix credential

#### Scenario: A granted token is scoped to what was approved

- **WHEN** a plugin was installed with a grant of read-only scopes and it attempts a write through the
  API
- **THEN** the write is refused as a permission failure

#### Scenario: Removing a plugin revokes its token

- **WHEN** a plugin holding a granted token is removed
- **THEN** that token no longer authenticates

### Requirement: A plugin receives the resolved connection target

tix SHALL resolve its connection target once, through the same layered resolution every command uses,
and SHALL pass the result to the plugin in its environment: whether the target is a local database or a
remote server, the address or DSN of that target, the resolved tenant, the output format the caller
asked for, the tix version, the name the plugin was invoked as, and the absolute path of the running
tix binary. A plugin SHALL NOT have to reimplement the resolution.

#### Scenario: A remote target is handed over

- **WHEN** the caller's target resolves to a server URL and a plugin is invoked
- **THEN** the plugin's environment names the remote mode and that server URL

#### Scenario: A local target is handed over

- **WHEN** the caller's target resolves to a local database and a plugin is invoked
- **THEN** the plugin's environment names the local mode and that database, and the operator was told
  at install time that a local target gives the plugin direct store access with no scope enforcement

#### Scenario: Re-invoking tix reaches the same target

- **WHEN** a plugin runs the tix binary named in its environment with no connection flags
- **THEN** that invocation resolves to the same target its caller used

#### Scenario: The output format is passed through

- **WHEN** a plugin is invoked under `-o json`
- **THEN** its environment names that format, and a plugin honouring the contract writes JSON

### Requirement: A plugin's exit status is tix's exit status

When a plugin runs, tix SHALL exit with the plugin's exit status unchanged, and SHALL NOT substitute a
code of its own. Failures tix detects before the plugin starts SHALL use the documented codes for an
unknown command or a refused invocation, and SHALL produce no plugin output.

#### Scenario: A plugin's failure code survives

- **WHEN** a plugin exits with a non-zero status
- **THEN** tix exits with that same status

#### Scenario: A plugin's success survives

- **WHEN** a plugin exits zero
- **THEN** tix exits zero

#### Scenario: A pre-execution refusal is distinguishable

- **WHEN** a plugin is refused for a digest mismatch
- **THEN** tix exits with its own failure code and the plugin produced no output

### Requirement: A built-in command is never shadowed

A plugin SHALL NOT be reachable under the name of a built-in command or alias. Dispatch to a plugin
SHALL happen only when no built-in command matches. Installation under a name already taken by a
built-in SHALL be allowed but SHALL warn, and listing SHALL mark such a plugin as unreachable.

#### Scenario: The built-in wins

- **WHEN** a plugin named `task` is installed and `tix task ls` is invoked
- **THEN** the built-in task listing runs and the plugin does not

#### Scenario: The collision is reported

- **WHEN** a plugin is installed under a built-in's name
- **THEN** the install warns, and listing marks the plugin unreachable and names the built-in in the way

#### Scenario: An alternative name restores reachability

- **WHEN** the same plugin is installed under a name no built-in uses
- **THEN** invoking that name runs it

### Requirement: Plugin commands stay out of the generated command table

The command summary table generated from the Cobra tree SHALL contain only built-in commands. A plugin
SHALL NOT contribute a row, so the generated table is identical on every machine regardless of what is
installed. The `tix plugin` command and its subcommands are built-ins and SHALL appear.

#### Scenario: An installed plugin adds no row

- **WHEN** the command table is generated on a machine with plugins installed
- **THEN** it is byte-identical to the table generated with none installed

#### Scenario: The committed table stays verifiable

- **WHEN** the guard comparing the committed table against the generated one runs with plugins installed
- **THEN** it passes

#### Scenario: The management command is in the table

- **WHEN** the command table is generated
- **THEN** it carries a row for `tix plugin`

### Requirement: Plugins are outside the capability registry

A plugin SHALL NOT be an entry in the capability registry, because it is not a service method and binds
no surface of its own. Registry parity SHALL be unaffected by what is installed. A plugin reaching the
service does so through bindings that are already registered.

#### Scenario: Parity is independent of installed plugins

- **WHEN** the registry parity checks run with plugins installed
- **THEN** their result is the same as with none installed

#### Scenario: A plugin's reach is the reach of existing bindings

- **WHEN** a plugin performs work against tix
- **THEN** it does so through the documented CLI or HTTP bindings, under the authorization those already
  enforce

### Requirement: Completion offers plugin names and nothing further

Shell completion SHALL offer the names of installed plugins alongside built-in commands, read from the
manifest. Completion SHALL NOT execute a plugin. A plugin's own arguments and flags SHALL NOT be
completed.

#### Scenario: Plugin names are completed

- **WHEN** completion is requested for the first word after `tix`
- **THEN** installed, reachable plugin names appear alongside built-in commands

#### Scenario: Completion runs nobody's code

- **WHEN** completion is requested with a plugin installed
- **THEN** the plugin's executable is not executed

#### Scenario: Plugin arguments are not completed

- **WHEN** completion is requested for an argument of a plugin command
- **THEN** no candidate is offered, and the built-in value completions for task references, project
  keys, tags and workflow states are not consulted
