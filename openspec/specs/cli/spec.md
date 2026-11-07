# cli Specification

## Purpose

Defines the command-line surface: its output formats, its streaming behaviour on large results, and the scripting contract callers depend on.

## Requirements

### Requirement: Streaming output for large results

The CLI SHALL provide an `ndjson` output format that emits one JSON object per line and writes each record as it is produced. A listing or an export SHALL NOT be held in memory in its entirety before the first byte is written.

#### Scenario: One object per line

- **WHEN** a listing is requested with `-o ndjson`
- **THEN** each record is a complete JSON object on its own line, parseable independently of the others

#### Scenario: Records are written as they are produced

- **WHEN** a large listing is streamed
- **THEN** output begins before the final record has been read, and memory use does not grow with the number of records

#### Scenario: Export streams

- **WHEN** a tenant is exported
- **THEN** the snapshot is written record by record rather than assembled first, so export is bounded by output size and not by available memory

#### Scenario: Import streams

- **WHEN** a snapshot is imported
- **THEN** it is read record by record, so importing does not require holding the whole snapshot in memory

### Requirement: Separation of data and diagnostics

The CLI SHALL write command data to standard output and SHALL write all diagnostics, progress, warnings, and errors to standard error, so that standard output can be piped without contamination.

#### Scenario: Piping data

- **WHEN** a list command is piped into another program while emitting warnings
- **THEN** only the data reaches the pipe and the warnings appear on the terminal

#### Scenario: Errors on standard error

- **WHEN** a command fails
- **THEN** the error message appears on standard error and standard output carries no error text

#### Scenario: Quiet mode

- **WHEN** `--quiet` is passed
- **THEN** non-essential diagnostics are suppressed while data output and the exit code are unchanged

### Requirement: Standard input for bodies and lists

Commands that accept a body or a list of references SHALL read from standard input when input is piped or when `-` is given as the argument.

#### Scenario: Body from stdin

- **WHEN** a comment body is piped into the comment command
- **THEN** the piped content is used as the body

#### Scenario: Refs from stdin

- **WHEN** a list of task references is piped into a command that accepts refs
- **THEN** the command operates on every reference read from standard input

#### Scenario: Explicit dash

- **WHEN** `-` is passed where a file or body is expected
- **THEN** the content is read from standard input

#### Scenario: Bulk pipeline

- **WHEN** the output of a list command is filtered and piped into a mutating command that accepts refs
- **THEN** each referenced task is processed and a per-reference result is reported

### Requirement: Output format selection

Every command that emits data SHALL support `-o table|json|yaml`. The default SHALL be a human-readable table and the JSON and YAML forms SHALL be stable and machine-parseable.

#### Scenario: JSON output

- **WHEN** `-o json` is passed to any data-emitting command
- **THEN** standard output is a single valid JSON document containing the full result

#### Scenario: YAML output

- **WHEN** `-o yaml` is passed
- **THEN** standard output is a valid YAML document containing the same fields as the JSON form

#### Scenario: Empty result

- **WHEN** a list command matches nothing and `-o json` is passed
- **THEN** an empty collection is emitted and the exit code is success

#### Scenario: Unknown format

- **WHEN** an unsupported value is passed to `-o`
- **THEN** the command fails with a usage error listing the supported formats

### Requirement: Stable machine-readable output

Field names and value encodings in the JSON and YAML forms SHALL be stable across releases, and SHALL NOT be reordered or renamed without a documented change.

#### Scenario: Field names unchanged

- **WHEN** the same command is run against a later release
- **THEN** existing field names and their value types are unchanged

#### Scenario: Timestamps in a fixed format

- **WHEN** a result contains a timestamp
- **THEN** it is emitted in a single documented format regardless of locale or terminal settings

#### Scenario: No decoration in machine formats

- **WHEN** `-o json` is used
- **THEN** the output contains no colour codes, headers, or padding

### Requirement: Colour and terminal awareness

The CLI SHALL disable colour when `NO_COLOR` is set, when output is not a terminal, or when colour is explicitly disabled, and SHALL never emit colour codes in machine-readable formats.

#### Scenario: NO_COLOR honoured

- **WHEN** `NO_COLOR` is set in the environment
- **THEN** output contains no ANSI colour sequences

#### Scenario: Redirected output

- **WHEN** standard output is redirected to a file
- **THEN** output contains no ANSI colour sequences

#### Scenario: Forced colour

- **WHEN** colour is explicitly forced while output is redirected
- **THEN** colour sequences are emitted as requested

### Requirement: Documented exit codes

The CLI SHALL use exit code 0 for success, 1 for a generic error, 2 for a usage error, 3 for not found or no task available, 4 for conflict or lease expired, 5 for an authentication or authorization failure, 6 for a failed precondition, and 7 when a system tix reads from, such as a sync source, could not be read. These codes SHALL be documented in help output.

#### Scenario: Success

- **WHEN** a command completes normally
- **THEN** it exits 0

#### Scenario: Usage error

- **WHEN** a command is invoked with an unknown flag or a missing required argument
- **THEN** it exits 2 and prints usage to standard error

#### Scenario: Failed precondition

- **WHEN** a command is refused because the data is in a state the operation forbids, such as a transition the workflow does not allow or deleting a task that still has subtasks
- **THEN** it exits 6 rather than the generic error code

#### Scenario: Empty queue

- **WHEN** a claim-next command finds no eligible task
- **THEN** it exits 3

#### Scenario: Lease expired

- **WHEN** a command presents an expired lease token
- **THEN** it exits 4

#### Scenario: Permission denied

- **WHEN** the actor is not permitted to perform the operation
- **THEN** it exits 5

#### Scenario: Upstream unreachable

- **WHEN** a sync run cannot reach the system it imports from
- **THEN** it exits 7

### Requirement: Exit codes derive from the service error taxonomy

Exit codes SHALL be produced by mapping the service error kind, and the same error kind SHALL always yield the same exit code regardless of which command produced it or which transport was used.

#### Scenario: Consistent mapping across commands

- **WHEN** a not-found error arises from two different commands
- **THEN** both exit with the same code

#### Scenario: Consistent mapping across transports

- **WHEN** the same failing operation is run against a local database and against a remote server
- **THEN** both invocations exit with the same code

### Requirement: Dry run for mutating commands

Every command that mutates state SHALL accept `--dry-run`, which reports exactly what would change and SHALL NOT write anything.

#### Scenario: Nothing written

- **WHEN** a mutating command is run with `--dry-run`
- **THEN** no domain row, audit entry, or event is created

#### Scenario: Planned changes reported

- **WHEN** a bulk mutation is run with `--dry-run`
- **THEN** the output lists each entity that would be created, updated, or skipped

#### Scenario: Machine-readable plan

- **WHEN** `--dry-run -o json` is used
- **THEN** the planned changes are emitted as structured data

#### Scenario: Validation still applies

- **WHEN** a dry run is given invalid input
- **THEN** the command fails with the same error it would produce without the flag

### Requirement: Command surface

The CLI SHALL provide the top-level commands `actor`, `artifact`, `audit`, `bundle`, `claim`, `comment`, `completion`, `config`, `connection`, `ctx`, `demo`, `dep`, `docs`, `doctor`, `domain`, `export`, `field`, `import`, `login`, `logout`, `member`, `project`, `prune`, `retention`, `serve`, `ssh`, `stats`, `status`, `sync`, `tag`, `task`, `tenant`, `theme`, `token`, `tui`, `update`, `user`, `version`, `watch`, `webhook`, and `workflow`.

#### Scenario: Unknown command

- **WHEN** an unrecognized command is invoked
- **THEN** the CLI exits 2 and suggests the closest matching command

#### Scenario: Version output

- **WHEN** `tix version` is run
- **THEN** the version is printed, and `-o json` yields it as structured data along with build metadata

### Requirement: The root help lists exactly the command surface

The root help SHALL list exactly the top-level commands of the command surface, so a command added to the tree is added here.

#### Scenario: Every group is reachable

- **WHEN** the root help is displayed
- **THEN** each of the listed command groups appears

### Requirement: Filter expression on the command line

The task listing command SHALL accept the same filter expression language the terminal interface's filter bar accepts, so an expression selects the same tasks from either surface. The expression SHALL be a conjunction of space-separated terms, SHALL support negating a term with a leading `-`, and SHALL support a weak match written with `~` in place of `:`.

#### Scenario: Negated term

- **WHEN** tasks are listed with the expression `-tag:ops`
- **THEN** only tasks lacking that tag are returned

#### Scenario: Weak match

- **WHEN** tasks are listed with the expression `title~api`
- **THEN** only tasks whose title contains that value are returned, whatever its case

### Requirement: A task filter expression adds to the flags and refuses what it cannot read

Flags given alongside the expression SHALL add to it rather than replace it. An expression the parser cannot read SHALL be a usage error, not an empty listing.

#### Scenario: Expression and flags combine

- **WHEN** tasks are listed with both an expression and a filter flag
- **THEN** only tasks satisfying both are returned

#### Scenario: Unreadable expression

- **WHEN** tasks are listed with an expression naming an unknown key, or applying the weak operator to a term that has no text
- **THEN** the command exits with a usage error rather than returning an empty listing

### Requirement: Event stream command exposes its resume cursor

The command that follows the event stream SHALL print each event's sequence number in every output format, SHALL accept a sequence number to resume after, and SHALL document its delivery guarantee as at-least-once with a cursor.

#### Scenario: Cursor on the default line

- **WHEN** the stream is followed in the default format
- **THEN** each line leads with the sequence number the consumer would resume from

#### Scenario: Structured output carries the whole event

- **WHEN** the stream is followed with a structured output format
- **THEN** each record carries the sequence number, actor, subject type and identifier, timestamp and full payload

#### Scenario: Resume after a sequence number

- **WHEN** the stream is followed resuming after a recorded sequence number
- **THEN** every matching event committed after that number is delivered, in order, before live delivery begins

### Requirement: Shell completion

The CLI SHALL generate completion scripts for bash, zsh, and fish, and those completions SHALL include dynamic completion of task references, project keys, tags, and statuses.

#### Scenario: Script generation

- **WHEN** `tix completion bash` is run
- **THEN** a valid bash completion script is written to standard output

#### Scenario: Dynamic task refs

- **WHEN** completion is requested for an argument that takes a task reference
- **THEN** existing task references are offered

#### Scenario: Dynamic statuses respect the workflow

- **WHEN** completion is requested for a status argument in a project with a custom workflow
- **THEN** the statuses offered are those defined by that project's workflow

#### Scenario: Completion never blocks the shell

- **WHEN** the configured target is unreachable while completing
- **THEN** completion returns no dynamic candidates promptly rather than hanging

### Requirement: Help with runnable examples

Every command's `--help` SHALL include at least one example that can be copied and run as shown, and SHALL describe the exit codes the command can produce.

#### Scenario: Example present

- **WHEN** `--help` is shown for any leaf command
- **THEN** at least one concrete example invocation is displayed

#### Scenario: Examples are valid

- **WHEN** the examples in help output are checked
- **THEN** each parses as a valid invocation of that command

#### Scenario: Exit codes documented

- **WHEN** help is shown for a command that can report not found or conflict
- **THEN** the corresponding exit codes are described

### Requirement: Generated command documentation

`tix docs` SHALL emit the full command tree as Markdown so that reference documentation is generated rather than maintained by hand.

#### Scenario: Full tree emitted

- **WHEN** `tix docs` is run
- **THEN** every command and subcommand, with its flags and examples, appears in the generated Markdown

#### Scenario: Output destination

- **WHEN** an output directory is supplied
- **THEN** the Markdown files are written there, and otherwise the documentation is written to standard output

#### Scenario: Documentation stays current

- **WHEN** a command or flag is added and the generated documentation in the repository is compared with freshly generated output
- **THEN** any difference is detected so stale documentation is caught

### Requirement: Global flags available on every command

Global flags including context selection, target overrides, output format, quiet, and discovery control SHALL be accepted on every command and SHALL behave identically wherever they appear in the argument order.

#### Scenario: Flag before the subcommand

- **WHEN** a global flag is placed before the subcommand
- **THEN** it takes effect

#### Scenario: Flag after the subcommand

- **WHEN** the same global flag is placed after the subcommand
- **THEN** it takes effect identically

### Requirement: Predictable and scriptable references

Commands that accept an entity SHALL accept a stable human-typed reference, and every command that creates an entity SHALL emit that entity's reference in a form usable as input to other commands.

#### Scenario: Created ref is reusable

- **WHEN** a task is created and its reference is captured from the output
- **THEN** that reference can be passed directly to another command

#### Scenario: Reference in machine output

- **WHEN** a create command is run with `-o json`
- **THEN** the reference appears as a discrete field rather than only inside a formatted message

#### Scenario: Unknown reference

- **WHEN** a command is given a reference that does not resolve
- **THEN** it exits 3 with a message naming the unresolved reference

### Requirement: Partial failure reporting in bulk operations

When a command operates on multiple references, it SHALL report a per-reference outcome and SHALL exit non-zero if any reference failed.

#### Scenario: Mixed results

- **WHEN** a bulk operation succeeds for some references and fails for others
- **THEN** each reference's outcome is reported and the command exits non-zero

#### Scenario: Machine-readable outcomes

- **WHEN** a bulk operation is run with `-o json`
- **THEN** the output contains one entry per reference with its outcome and, on failure, the error code

#### Scenario: All succeed

- **WHEN** every reference in a bulk operation succeeds
- **THEN** the command exits 0

### Requirement: Activity filter expression on the command line

The audit listing command SHALL accept a filter expression over the activity vocabulary, spelled the way the task filter expression is spelled, and SHALL accept the same expression the terminal interface's activity filter accepts. The expression SHALL select on actor, subject kind, action, source and free text, SHALL support negating any of those with a leading `-`, and SHALL add to the command's own flags rather than replace them.

#### Scenario: Select by kind

- **WHEN** audit entries are listed with the expression `kind:project`
- **THEN** only entries whose subject is a project are returned

#### Scenario: Select by source

- **WHEN** audit entries are listed with the expression `source:web`
- **THEN** only entries recorded as arriving through the browser are returned

#### Scenario: Free text searches what the entry recorded

- **WHEN** audit entries are listed with a bare word
- **THEN** only entries whose action, kind, source or before and after snapshots contain that word are returned

#### Scenario: Negated term

- **WHEN** audit entries are listed with the expression `-kind:task`
- **THEN** entries about tasks are excluded

#### Scenario: Expression and flags combine

- **WHEN** audit entries are listed with both an expression and a filter flag that contradict each other
- **THEN** nothing is returned, because a filter narrows what the flags selected and never widens it

#### Scenario: Free text does not stop at the first page

- **WHEN** a free-text expression discards every entry on the first page read
- **THEN** further pages are read, up to a bound, and the command reports on its diagnostic stream how much was discarded and the cursor to resume from

### Requirement: An unreadable activity filter expression is a usage error

An activity filter expression the parser cannot read, or one naming a source that is not recorded, SHALL be a usage error rather than an empty listing.

#### Scenario: Unreadable expression

- **WHEN** audit entries are listed with an expression naming a key the activity vocabulary does not have, or a source that is not recorded
- **THEN** the command exits with a usage error rather than returning an empty listing

### Requirement: Completion installs itself where the shell looks

The command line SHALL install a completion script to the location the named shell loads completions from,
for bash, zsh and fish, and SHALL report the path it wrote.

The shell SHALL be taken from the environment when it is not given explicitly, and an explicit choice SHALL
override it. An unsupported shell SHALL be refused.

#### Scenario: Installing for the running shell

- **WHEN** completion is installed with no shell named and the environment names a supported shell
- **THEN** the script is written to that shell's completion directory and the path is reported

#### Scenario: An explicit shell overrides the environment

- **WHEN** completion is installed with a shell named explicitly
- **THEN** that shell's script is written, whatever the environment says

#### Scenario: A dry run writes nothing

- **WHEN** completion is installed as a dry run
- **THEN** the path that would be written is reported and no file is created or changed

#### Scenario: An unsupported shell is refused

- **WHEN** completion is installed for a shell that is not supported
- **THEN** the command fails with the exit code for unusable input and names the shells that are supported

### Requirement: Completion installation stays in the user's own directories and can be repeated

Completion installation SHALL write only under the invoking user's own directories, and SHALL NOT modify
shell startup files. Where a shell needs something further before completion works, the command SHALL say
so.

Installing completion twice SHALL succeed and leave the same result.

#### Scenario: Installing twice is not an error

- **WHEN** completion is installed and then installed again
- **THEN** the second run succeeds and the file matches the first

#### Scenario: Uninstalling removes what was written

- **WHEN** completion is uninstalled
- **THEN** the file this command writes is removed, and removing it when absent is reported rather than failed

### Requirement: The binary can replace itself with a published release

The command line SHALL replace the running binary with a release built for its own operating system and
architecture, and SHALL report the version it moved from and to.

The replacement SHALL be atomic: the new binary SHALL be written beside the target and moved into place,
so that an interrupted or failed update never leaves a partially written file where the binary was.

#### Scenario: Updating to the current release

- **WHEN** a newer release exists for this platform and the binary is one this command may replace
- **THEN** the archive for this platform is downloaded, its checksum verified, and the binary replaced,
  and the output names the version before and after

#### Scenario: An interrupted download leaves the binary alone

- **WHEN** the download or extraction fails part way through
- **THEN** the binary that was already there still runs and no partial file is left in its place

#### Scenario: Already current

- **WHEN** the running version is the newest release
- **THEN** the command reports that it is current, writes nothing, and succeeds

### Requirement: A self-update verifies the published checksum

The command line SHALL verify the downloaded archive against the checksum published with that release,
and SHALL refuse to install an archive whose checksum does not match or that the checksum file does not
list.

#### Scenario: A checksum that does not match is refused

- **WHEN** the downloaded archive's checksum differs from the one published for it
- **THEN** nothing is installed, the existing binary is untouched, and the failure says the checksum did
  not match

#### Scenario: An archive the checksum file does not list is refused

- **WHEN** the published checksum file carries no entry for the archive that was downloaded
- **THEN** nothing is installed and the failure says the archive was not listed

### Requirement: It refuses what is not its to replace

The command SHALL determine how the running binary was installed, and SHALL refuse rather than overwrite a
binary owned by another tool, naming the command that would do the upgrade instead.

A target that cannot be written SHALL be reported before anything is downloaded.

#### Scenario: A binary installed with the Go toolchain

- **WHEN** the running binary was installed by the Go module toolchain
- **THEN** the command refuses and names the `go install` command that would upgrade it

#### Scenario: A binary a package manager owns

- **WHEN** the running binary resolves inside a path a package manager controls
- **THEN** the command refuses and says the binary is managed elsewhere

#### Scenario: An unwritable destination is reported first

- **WHEN** the directory holding the binary cannot be written by this user
- **THEN** the command fails before downloading anything and says the destination is not writable

### Requirement: Checking does not install

The command SHALL offer a mode that reports whether a newer release exists and exits without downloading
or writing anything, and SHALL report the absence of a newer release as success rather than as an error.

#### Scenario: Checking when a newer release exists

- **WHEN** the check mode runs and a newer release exists
- **THEN** it names that version and no file is created, downloaded or replaced

#### Scenario: Checking when current

- **WHEN** the check mode runs and this binary is the newest release
- **THEN** it says so and succeeds

### Requirement: A specific version may be named

The command SHALL accept a release to install instead of the newest, including one older than the running
binary, and SHALL refuse a version that has no release for this platform.

#### Scenario: Installing a named release

- **WHEN** a version is named and a release exists for it on this platform
- **THEN** that release is installed, whether it is newer or older than the running binary

#### Scenario: A version with no release for this platform

- **WHEN** a version is named that publishes nothing for this operating system and architecture
- **THEN** nothing is installed and the failure names the platform it looked for

### Requirement: A second factor on the command line

When a tenant requires a second factor for the command line, `tix login` SHALL
prompt for a TOTP or recovery code after the password is accepted, and SHALL
mint a session only once the code is accepted.

The code SHALL NOT be accepted as a command-line flag or as an environment
variable. A live second factor passed as an argument lands in shell history and
in the process table, and the only motivation for wanting it is scripting, which
API tokens already serve.

#### Scenario: Prompted after the password

- **WHEN** a user runs `tix login` against a tenant requiring a factor for the command line
- **THEN** the password is taken first, a code is prompted for, and a session is minted only on success

#### Scenario: No flag carries the code

- **WHEN** a user looks for a way to supply the code non-interactively
- **THEN** no such flag or environment variable exists, and the help text points at API tokens for unattended use

#### Scenario: A non-interactive terminal

- **WHEN** `tix login` runs without a terminal in a tenant requiring a factor
- **THEN** it fails with an error naming API tokens as the unattended path, rather than hanging on a prompt

### Requirement: Managing enrolment from the command line

`tix user totp` SHALL offer enrolment, status and removal of one's own
enrolment, and SHALL allow an administrator to clear another user's.

Enrolment SHALL present the `otpauth://` URI and, where the terminal can render
it, a scannable code, and SHALL display the recovery codes exactly once.

#### Scenario: Enrol from the terminal

- **WHEN** a user runs the enrolment command and submits a correct code
- **THEN** the enrolment is confirmed and ten recovery codes are printed once

#### Scenario: Status reports enrolment without revealing the secret

- **WHEN** a user asks for their TOTP status
- **THEN** it reports whether they are enrolled and how many recovery codes remain, and never prints the secret

#### Scenario: Administrative clearing

- **WHEN** an administrator clears another user's enrolment
- **THEN** the command succeeds, and a user without the administrative scope attempting the same is refused

### Requirement: Starring from the command line

The CLI SHALL offer `tix task star <ref>` and `tix task unstar <ref>`. `tix task ls` SHALL mark a
starred task in its table output, `tix task show` SHALL state whether the task is starred, and the
structured output of both SHALL carry `starred: true` for a starred task. `tix task ls` SHALL
accept `--starred` and `--starred=false`.

#### Scenario: Star then list

- **WHEN** `tix task star OPS-3` is run and then `tix task ls`
- **THEN** OPS-3 is the first row and is marked starred

#### Scenario: Structured output

- **WHEN** `tix task show OPS-3 -o json` is run for a starred task
- **THEN** the document carries `"starred": true`

### Requirement: The command line sets, clears and shows a repeat rule

`tix task add` SHALL accept `--repeat RULE`. `tix task edit` SHALL accept `--repeat RULE` to set or replace
the rule and `--no-repeat` to remove it, and SHALL refuse both together as a usage error. `tix task show`
SHALL state the rule and the successor's reference when there is one; the table listing SHALL mark a
recurring task; JSON and YAML output SHALL carry `recurrence` and `recurrence_next_id`.

#### Scenario: Add with a rule

- **WHEN** `tix task add "Pay rent" --due 2026-11-01 --repeat monthly` is run
- **THEN** the task is created carrying `every month`

#### Scenario: Both flags

- **WHEN** `tix task edit REF --repeat daily --no-repeat` is run
- **THEN** it exits with a usage error and the task is unchanged

#### Scenario: Show

- **WHEN** `tix task show` is run on a finished occurrence that spawned
- **THEN** the output states its rule and names the successor's reference

### Requirement: A command seeds a demonstrable backlog

The command line SHALL provide a command that fills a database with a backlog spanning a configurable
window, so that the screens can be shown doing the thing they exist for rather than empty.

#### Scenario: Seeding a scratch database

- **WHEN** the command is run against a database holding no tasks or users
- **THEN** projects, actors, custom field definitions, tasks, comments and dependencies are written across
  the requested window, and the command reports what it wrote

#### Scenario: A database that already holds work is refused

- **WHEN** the command is run against a database that already holds tasks or users
- **THEN** nothing is written, and the refusal says to re-run with the reset flag or to point at a scratch
  database

#### Scenario: Replacing seeded data

- **WHEN** the command is run with the reset flag
- **THEN** the tasks and projects already there are removed before the new backlog is written, and the
  accounts are kept so that the seed can put its credentials back on them

#### Scenario: The seeded tenant shows only the lists it wrote

- **WHEN** the command is run against a database carrying the starter lists a fresh installation is given
- **THEN** the starter lists that hold no task are removed, so the projects screen and the project filters
  name only the lists the backlog works out of

### Requirement: Seeded data is written through the service on an advancing clock

The seeded data SHALL be written through the ordinary service calls on a clock the command advances, not
by writing rows or importing a snapshot. Statistics attribute a completion by matching the transition
entry written in the same instant as the completion, so data written behind the service leaves every
leaderboard and lead-time figure empty, which is the opposite of the point.

#### Scenario: The statistics screens have something to show

- **WHEN** statistics are read over the seeded window
- **THEN** completions fall on many different days, the leaderboard names more than one actor, and the
  median lead time is not zero

### Requirement: The seeded backlog varies in content, days, actors and lead times

The seeded backlog SHALL carry descriptions, tags, priorities, assignees, due dates and custom fields,
completions spread unevenly across many days and performed by several actors, and lead times that vary.
A completion on every day of the window is as useless to the throughput chart as no completion at all,
because every value is then also the maximum the bars are scaled against.

#### Scenario: The throughput chart reads as work rather than a flat wall

- **WHEN** statistics are read over each of the windows the statistics screen offers
- **THEN** the per-day completion counts within that window differ from one another, so the chart draws
  bars of differing lengths rather than one identical bar per day

#### Scenario: A dropped claim falls on the first page of the default ordering

- **WHEN** the seeded backlog is listed in the default urgency ordering
- **THEN** a task carrying a recently expired claim falls inside the first page, so the expiry badge and
  the pager can be seen on one screen

#### Scenario: A dropped claim is visible in the seeded backlog

- **WHEN** the seeded backlog is read
- **THEN** a small number of tasks report a claim that expired within the evidence window, at least one of
  them having been claimed more than once, so the state a lease expiry leaves behind can be seen without
  arranging it by hand

### Requirement: The seeded demonstration can be signed in to

The seeding command SHALL leave at least one credentialed account that can sign in to the browser
interface, created through the same service call the user-creation command makes, so that a seeded
demonstration is one identity per person rather than an account beside the actor that wrote the history.

#### Scenario: Signing in to a freshly seeded database

- **WHEN** a database is seeded and the reported email and password are presented to the sign-in path
- **THEN** a session is issued, and a password that is not the reported one is rejected

### Requirement: Seeded sign-in accounts are identities the fixture writes history as

The accounts SHALL be identities the fixture already writes history as, at least one of them an
administrator and at least one holding a different role, so the screens can be shown as more than one
kind of user. An account that the fixture drives as an agent SHALL NOT be given a password, because an
agent authenticates with the token minted beside it.

#### Scenario: Agents hold tokens rather than passwords

- **WHEN** the seeded agents are examined
- **THEN** none of them is reported as a sign-in, and none of them can sign in with the seeded password

#### Scenario: A handle held by an actor with no account

- **WHEN** the command is run against a database where a seeded handle belongs to an actor no account
  stands behind
- **THEN** the seed is refused, naming the handle, rather than writing a history whose authors nobody can
  sign in as

### Requirement: The seeded password is a documented default the caller may replace

The password SHALL be a documented default that the caller may replace with a flag, because an account
nobody can guess the password of leaves the demonstration as unreachable as no account at all. The seeded
data is demonstration data by construction, and a database that already holds work or accounts is
refused before any of it is written.

#### Scenario: Re-seeding leaves a working sign-in

- **WHEN** the command is run with the reset flag against a database seeded before
- **THEN** the accounts are re-credentialed with the password the run reports, and signing in with it
  succeeds

### Requirement: The seed reports the credentials it left in every output format

The command SHALL report the credentials it left in every output format, so that the person who ran it is
told how to open what it wrote without reading the documentation.

#### Scenario: The credentials are reported

- **WHEN** the seed finishes
- **THEN** the table output names the email, password and role of every account that can sign in, and the
  machine-readable formats carry the same fields

### Requirement: Durations are rendered for a reader

Every surface SHALL render a duration it shows to a person in a form that drops precision as the
magnitude rises, and SHALL NOT show the machine rendering of a duration on a screen meant to be read.

The rendering SHALL belong to the duration type, so the command line, the terminal interface and the
browser cannot drift apart on the same figure. The machine-readable output formats SHALL keep the full
precision, because a snapshot is built from them.

#### Scenario: A lead time counted in weeks

- **WHEN** a statistics screen shows a lead time of 384 hours
- **THEN** it reads as a count of days rather than as hours, minutes and fractional seconds

#### Scenario: The machine formats are unchanged

- **WHEN** the same figure is written as JSON or YAML
- **THEN** it carries the full precision

### Requirement: A duration the product prints can be typed back

The product SHALL read durations in one vocabulary everywhere a person supplies one, and that vocabulary
SHALL include everything it renders. A duration rendered for a reader SHALL parse, and parsing it and
rendering it again SHALL produce the identical text.

The machine rendering SHALL stay Go's syntax, because snapshots and JSON round-trip through it.

#### Scenario: The reader's rendering is valid input

- **WHEN** a duration is rendered for a reader as `16d 1h`
- **THEN** that text parses, and rendering the result again gives `16d 1h`

#### Scenario: The snapshot format is unchanged

- **WHEN** a duration is written to a snapshot or to JSON
- **THEN** it carries Go's syntax, which Go's own parser still reads

### Requirement: The duration vocabulary is Go's syntax with a day unit

The vocabulary SHALL be Go's duration syntax extended with a day unit of exactly twenty-four hours, and
SHALL accept whitespace between terms, because that is the form the reader's rendering takes. It SHALL NOT
add a unit nothing renders, since a unit accepted but never printed is the same asymmetry facing the other
way.

#### Scenario: A statistics window in days

- **WHEN** `--window 30d` is passed to the statistics command
- **THEN** the window is thirty days, and the refusal for an unreadable value names a form that works

### Requirement: One duration grammar serves every surface

The grammar SHALL belong to the duration type, so the command line, the HTTP interface, the browser
and the configuration reader cannot drift apart on what a duration is.

#### Scenario: A retention window as the screen shows it

- **WHEN** a retention window is given as `30d`
- **THEN** it is accepted as seven hundred and twenty hours, on the command line, over HTTP and in
  configuration

### Requirement: A failed listing writes no answer to standard output

A listing command that fails SHALL NOT write a complete document to standard output. A document that
parses is an answer, and a consumer reading only standard output, which is the ordinary shape of a
pipeline, cannot tell an empty answer from a failure it never saw.

The failure SHALL still be reported on standard error with the exit status of its kind.

#### Scenario: A listing that fails before it writes

- **WHEN** a listing is filtered by an assignee that does not resolve, in any output format
- **THEN** standard output is empty, the failure is on standard error, and the exit status is the one for
  the failure's kind

### Requirement: A listing that fails part way leaves its document unterminated

Where records were already streamed before the failure, the command SHALL NOT complete the document: a
bracketed format is left unterminated, so a consumer parsing standard output gets a syntax error rather
than a short listing it would believe. A record-oriented format has no terminator to withhold, and the
records already written stand, each true on its own.

#### Scenario: A listing that fails part way through

- **WHEN** a bracketed listing fails after records have been written
- **THEN** the document is left unterminated and cannot be parsed as a complete listing

### Requirement: The binary carries its own zone database

The binary SHALL embed the zone database rather than depend on the host providing one, so that a reader's
chosen timezone is honoured on any base image.

#### Scenario: A base image with no tzdata

- **WHEN** the binary runs where the host has neither a zone database nor Go's bundled archive
- **THEN** a named timezone still resolves, rather than falling back to the deployment default without
  telling anyone

### Requirement: Project icon flags

`tix project create` and `tix project edit` SHALL accept `--icon` for a text icon, `--icon-name`
for a built-in icon and `--icon-file` for a path to an image to upload, and SHALL refuse more than
one of them with a non-empty value. `tix project icon KEY` SHALL write the project's uploaded image
to a file named with `--out`, or to standard output when it is not a terminal.

#### Scenario: Built-in icon from the command line

- **WHEN** `tix project edit infra --icon-name rocket` is run
- **THEN** the project carries the built-in rocket icon and `tix project ls` shows its fallback glyph and name

#### Scenario: Image from the command line

- **WHEN** `tix project edit infra --icon-file logo.png` is run and then `tix project icon infra --out copy.png`
- **THEN** `copy.png` holds the same bytes as `logo.png`

#### Scenario: Two kinds are refused

- **WHEN** `tix project create web Web --icon W --icon-name globe` is run
- **THEN** the command fails with exit code 2 and creates nothing

### Requirement: A custom project colour on a terminal

Where the CLI or the terminal interface draws a project colour that is a custom value, it SHALL draw
that colour itself on a terminal that declares truecolor, and the nearest colour the terminal can
show otherwise. With colour disabled it SHALL print the hexadecimal value as text.

#### Scenario: Truecolor terminal

- **WHEN** a project with a custom colour is listed with colour enabled and `COLORTERM=truecolor`
- **THEN** its swatch is drawn with a 24-bit foreground of that colour

#### Scenario: Sixteen colour terminal

- **WHEN** a project with a custom colour is listed with colour enabled and no truecolor declared
- **THEN** its swatch is drawn in the palette colour nearest to it

#### Scenario: No colour

- **WHEN** a project with a custom colour is listed with colour disabled
- **THEN** the colour column holds the hexadecimal value and no escape sequence

### Requirement: Every input the API accepts can be set from the command line

Every field of an input the HTTP API decodes SHALL be settable from the command bound to the same
operation, by an argument or a flag, unless the field is set by the command itself for a recorded
reason. A list command that reports a next cursor SHALL accept `--cursor` to continue from it.

#### Scenario: A custom field default

- **WHEN** `tix field put infra points --type int --default 3` is run and a task is then added to
  `infra` without a value for `points`
- **THEN** the task carries `points` equal to 3

#### Scenario: A text default that looks like a number

- **WHEN** `tix field put infra code --default 42` is run for a string field
- **THEN** the default is stored as the text `42`

#### Scenario: A move at a stale version

- **WHEN** `tix task mv infra-1 doing --version 7` is run and the task is at another version
- **THEN** the command exits 4 and the task is not moved

#### Scenario: A version with several tasks

- **WHEN** `tix task mv infra-1 infra-2 doing --version 7` is run
- **THEN** the command exits 2 without moving either task

#### Scenario: Following a cursor

- **WHEN** `tix project ls --limit 1` reports a next cursor and is run again with `--cursor` set to it
- **THEN** the second run lists the next project rather than the first again

### Requirement: View arrangement commands

The CLI SHALL provide `tix view ls VIEW` listing the caller's preferences on a view, `tix view set
VIEW GROUP` with `--hidden`, `--collapsed` and `--starred`, each applied only when given and
accepting `=false`, `tix view order VIEW GROUP...` putting the named groups first in that order, and
`tix view reset VIEW [GROUP]`. `set` with no flag SHALL be refused as a usage error.

#### Scenario: Hide a lane from the command line

- **WHEN** an operator runs `tix view set lanes homelab --hidden`
- **THEN** the lanes view in the browser no longer draws the `homelab` lane for that actor

#### Scenario: Set with nothing to change

- **WHEN** an operator runs `tix view set board done`
- **THEN** the command fails with a usage error and writes nothing

### Requirement: Deleting a project from the command line names the fate of its tasks

`tix project rm KEY` SHALL accept `--move-tasks-to OTHER` and `--delete-tasks`, which SHALL be mutually exclusive.
Without either, deleting a project that holds tasks SHALL exit 6 with a message naming the task count and both
flags. `--dry-run` SHALL print the plan, including the status mapping for a move, and write nothing. A successful
deletion SHALL report how many tasks were moved or deleted.

#### Scenario: Refused without a choice

- **GIVEN** a project holding tasks
- **WHEN** `tix project rm KEY` runs without either flag
- **THEN** it exits 6 and the message names `--move-tasks-to` and `--delete-tasks`

#### Scenario: Moving from the command line

- **WHEN** `tix project rm OLD --move-tasks-to NEW` runs
- **THEN** OLD is gone and its tasks are listed under NEW

#### Scenario: Both flags are a usage error

- **WHEN** both flags are passed
- **THEN** the command exits 2 and writes nothing

### Requirement: The project edit command changes the key

`tix project edit KEY` SHALL accept `--key NEW` to change the project's key, alongside the flags that
change its other attributes. An invalid key SHALL exit with the validation code and a key held by
another project SHALL exit with the conflict code. Every command that takes a project or a task
reference SHALL accept an old key of a renamed project, and SHALL print the current key and
reference.

#### Scenario: Renaming from the command line

- **WHEN** `tix project edit infra --key ops` runs
- **THEN** the project is printed with key `ops`

#### Scenario: An old reference on the command line

- **GIVEN** project `infra` was renamed to `ops`
- **WHEN** `tix task show infra-8` runs
- **THEN** the task is printed with reference `ops-8`

#### Scenario: A taken key exits with the conflict code

- **WHEN** `tix project edit web --key infra` runs and `infra` belongs to another project
- **THEN** the command exits with code 4

### Requirement: Copying from the command line

The CLI SHALL offer `tix task copy REF` with `--to PROJECT`, `--title`, `--keep-status`,
`--with-comments`, `--with-subtasks`, `--with-deps`, `--link` and `--dry-run`, and SHALL print
the copy the way `tix task show` prints a task. `tix task show` SHALL name the task a copy was
copied from.

#### Scenario: Copy to another project

- **WHEN** `tix task copy OPS-3 --to WEB --with-subtasks` is run
- **THEN** the output names a new WEB task and its subtasks exist in WEB

#### Scenario: Dry run

- **WHEN** `tix task copy OPS-3 --to WEB --dry-run` is run
- **THEN** the plan is printed and no task is created

### Requirement: The command line lists the trash

`tix task ls` SHALL accept `--trashed`, listing only soft-deleted tasks, equivalent to
`--filter is:trashed`.

#### Scenario: Listing the trash

- **GIVEN** a deleted task and a live one
- **WHEN** `tix task ls --trashed` runs
- **THEN** only the deleted task is printed

### Requirement: Bulk actions from the command line

The CLI SHALL offer `tix task bulk ACTION REF...`, where ACTION is one of `move`, `priority`,
`tag-add`, `tag-remove`, `assign`, `star`, `unstar`, `copy`, `delete` and `restore`, taking
`--to`, `--priority`, `--tag`, `--assignee`, `--project` and `--cascade` for the argument, `-`
as a reference to read references from standard input, and sending one request. It SHALL print
one outcome per task and exit non-zero when any task was refused.

#### Scenario: One request for many tasks

- **WHEN** `tix task bulk priority OPS-1 OPS-2 --priority high` is run
- **THEN** both tasks have priority high and the output lists two applied outcomes

### Requirement: Many references wherever one is taken for a change

`tix task edit` and `tix task copy` SHALL accept many references and apply the same change to
each, refusing `--version` and `--title` when more than one is given. `tix tag rm` SHALL take
`REF... TAG`. `tix tag add` SHALL accept a comma separated list of references, or `-` to read
them from standard input, in its reference position.

#### Scenario: Edit many tasks

- **WHEN** `tix task edit OPS-1 OPS-2 --priority low` is run
- **THEN** both tasks have priority low and one outcome per task is printed

#### Scenario: Tag many tasks

- **WHEN** `tix tag add OPS-1,OPS-2 ops` is run
- **THEN** both tasks carry the tag `ops`

### Requirement: Task listing as CSV

`tix task ls` SHALL accept `-o csv` and print a header row and one row per task in the columns
`ref, project, title, status, priority, assignee, tags, due, created, updated`, with the
assignee as a handle when it resolves, escaped by RFC 4180 and with the formula guard. Other
commands SHALL refuse `-o csv` with a usage error.

#### Scenario: CSV listing

- **WHEN** `tix task ls -o csv` is run
- **THEN** the first line is `ref,project,title,status,priority,assignee,tags,due,created,updated`

#### Scenario: CSV elsewhere

- **WHEN** `tix project ls -o csv` is run
- **THEN** the command exits with a usage error naming the formats it accepts

### Requirement: Saved filter commands

The CLI SHALL provide `tix filter ls`, `tix filter show NAME`, `tix filter save NAME` with `--filter`,
`--view`, `--sort` and `--shared`, `tix filter set NAME` changing only the flags given (refusing none as a
usage error), `tix filter rename NAME NEW-NAME`, `tix filter order NAME...` and `tix filter rm NAME`.
`tix task ls` SHALL accept `--saved NAME`, starting from that filter's expression and sort with the other
flags narrowing it, and `--mine`, meaning `assignee:me`.

#### Scenario: List from a saved filter

- **GIVEN** a saved filter `ops` holding `tag:ops` and sort `priority`
- **WHEN** an operator runs `tix task ls --saved ops --status todo`
- **THEN** the listing is the `ops` tasks in state `todo`, sorted by priority

#### Scenario: Set with nothing to change

- **WHEN** an operator runs `tix filter set ops`
- **THEN** the command fails with a usage error and writes nothing

### Requirement: Quick-add on the command line

`tix task add` SHALL take `--parse`, which reads the title through the quick-add parser,
resolving relative dates in the local time zone. Without `--parse` the title SHALL be stored
literally, so a script passing a title it did not write is never rewritten. An explicit flag
that contradicts the line SHALL be refused with exit code 2; the configured default project
SHALL apply only when the line names none.

#### Scenario: Parsed add

- **WHEN** `tix task add --parse 'Fix login !high #auth'` is run
- **THEN** the task is titled `Fix login`, has priority high and carries the tag `auth`

#### Scenario: Literal by default

- **WHEN** `tix task add 'Fix login !high #auth'` is run
- **THEN** the task is titled `Fix login !high #auth` with normal priority and no tags

#### Scenario: Unknown handle

- **WHEN** `tix task add --parse 'Fix login @nobody'` is run and no actor has that handle
- **THEN** the command exits 3 and creates nothing

### Requirement: The command line ticks and clears a checklist item

`tix task check REF N` SHALL tick checklist item N of the task's description and `tix task uncheck REF N`
SHALL clear it, each taking `--version` for optimistic locking and `--dry-run`. A stale version SHALL
exit `4`, and an item number the description does not have, or an N that is not a positive number, SHALL
exit `2`.

#### Scenario: Ticking the third item

- **GIVEN** a task whose body has three open items
- **WHEN** `tix task check default-1 3` runs
- **THEN** only the third item is ticked

### Requirement: Task show states the checklist

In table output `tix task show` SHALL state, on standard error, how many of the description's checklist
items are done, when it has any; JSON and YAML output SHALL carry the `checklist` field.

#### Scenario: The count is stated

- **GIVEN** a task with one of three items ticked
- **WHEN** `tix task show` runs
- **THEN** it prints `checklist: 1/3 done`

### Requirement: Calendar feed commands

The CLI SHALL provide `tix calendar show`, `tix calendar create [--filter EXPR]` (alias `url`),
`tix calendar rotate`, `tix calendar filter [EXPR]`, where no expression restores the default, and
`tix calendar revoke`. `create` and `rotate` SHALL print the feed URL, absolute when connected to a
server.

#### Scenario: Create a feed from the command line

- **WHEN** an operator connected to a server runs `tix calendar create --filter 'tag:ops'`
- **THEN** the command prints a URL that answers the feed without a credential

#### Scenario: Show never prints the secret

- **WHEN** an operator runs `tix calendar show -o json`
- **THEN** the output holds the filter and creation time and no secret

### Requirement: tix inbox

The CLI SHALL offer `tix inbox ls` (with `--unread`, `--limit` and `--cursor`), `tix inbox read ID`,
`tix inbox unread ID`, `tix inbox read-all`, `tix inbox count` and `tix inbox prefs` (with `--mute`
and `--unmute` to change the muted kinds), each with the table, JSON and YAML output every command
has.

#### Scenario: Listing unread items

- **WHEN** a user runs `tix inbox ls --unread`
- **THEN** each unread item is printed with its task, actor, kind and age

#### Scenario: Muting a kind

- **WHEN** a user runs `tix inbox prefs --mute comment`
- **THEN** `comment` is listed as muted and comments on watched tasks no longer reach them

### Requirement: tix inbox channel

The CLI SHALL offer `tix inbox channel ls`, `add NAME URL` (with `--format`, `--kinds`, `--off`),
`edit ID` (with `--name`, `--url`, `--format`, `--kinds`, `--active`), `rm ID`, `test ID`,
`deliveries ID` and `redeliver DELIVERY`.

#### Scenario: Testing a channel turned off

- **WHEN** a user runs `tix inbox channel test` on a channel turned off
- **THEN** the command exits 6

### Requirement: Snooze from the command line

`tix task snooze REF UNTIL` SHALL snooze a task, taking `later`, `tomorrow`, `next-week`, a
duration such as `2h` or `3d`, a date, or a timestamp, read in the local time zone.
`tix task unsnooze REF` SHALL unsnooze it, and `tix task wake` SHALL run one wake pass and print
how many tasks woke. `tix task show` SHALL say until when a snoozed task sleeps.

#### Scenario: Snooze until tomorrow

- **WHEN** a user runs `tix task snooze OPS-3 tomorrow`
- **THEN** OPS-3 is snoozed until 09:00 tomorrow local time and `tix task ls` no longer lists it

#### Scenario: Unparseable time

- **WHEN** a user runs `tix task snooze OPS-3 someday`
- **THEN** the command fails as a usage error

### Requirement: Template commands

The CLI SHALL offer `tix template ls`, `tix template save REF --name NAME [--tenant-wide]
[--with-subtasks] [--due-in D]`, `tix template show NAME`, `tix template edit NAME` with a flag
per editable field, `tix template rm NAME` and `tix template use NAME [--project P] [--title T]
[--due-in D]`.

#### Scenario: Use from the command line

- **WHEN** a user runs `tix template use release --due-in 3d`
- **THEN** the command prints the new task's reference and exits 0

### Requirement: The command line sets and reports the trash

`tix tenant edit` SHALL accept `--trash on|off`, refusing any other value as a usage error. `tix task
trash` SHALL report whether the trash is on and how many tasks it holds. With the trash off, `tix task
rm` SHALL keep its flags and SHALL NOT prompt, and SHALL say on stderr that the delete is permanent.

#### Scenario: Turning the trash off from the shell

- **WHEN** `tix tenant edit default --trash off` runs and then `tix task rm default-1`
- **THEN** stderr says the delete is permanent, and `tix task ls --trashed` exits 6

### Requirement: Tables and text draw keys and references upper case

The CLI's table output and its human-readable text SHALL draw project keys and task references upper
case (`INFRA`, `INFRA-8`). JSON, YAML, NDJSON and CSV output SHALL carry the stored lower-case value
unchanged. Every argument, flag and filter term that names a project or a task SHALL accept any
case.

#### Scenario: Table and JSON

- **WHEN** a user runs `tix task ls` and `tix task ls -o json` in a project `infra`
- **THEN** the table shows `INFRA-1` and the JSON `ref` is `infra-1`

#### Scenario: Any case in

- **WHEN** a user runs `tix task show INFRA-1`, `tix task add --project INFRA` or `tix task ls --filter project:INFRA`
- **THEN** each reaches the project `infra` and its task

### Requirement: The command line manages housekeeping

`tix cleanup show [--project KEY]` SHALL print the tenant policy, the project override and the effective
rules. `tix cleanup set [--project KEY] [--done V] [--cancelled V] [--trash V]` SHALL take `off`, a number of
days such as `30` or `30d`, and with `--project` also `inherit`. `tix cleanup preview [--project KEY]` SHALL
report what would be acted on now, and `tix cleanup run` SHALL run one bounded pass.

#### Scenario: Previewing a project

- **WHEN** `tix cleanup preview --project OPS` runs
- **THEN** it prints how many done, cancelled and trashed tasks would be acted on, and changes nothing

### Requirement: The command line sets and shows lease length

`tix task edit REF --lease D` SHALL set the task's lease and `--lease ""` SHALL clear it.
`tix project edit KEY --lease-default D --lease-max D` SHALL set the project's lease bounds and `""` SHALL inherit
the tenant's. `tix task show` SHALL print how long a claim would last and where that comes from, and claim output
SHALL report the length granted and its level.

#### Scenario: Showing where a lease comes from

- **WHEN** a project's lease default is 2h, its task sets none, and `tix task show` runs on the task
- **THEN** it prints `claims last 2h, from the project`

### Requirement: The command line logs in with an SSH key

`tix login --ssh-key` SHALL sign the server's challenge with each key in `ssh-agent`, then each default
key file under `~/.ssh`, until one is accepted, and print the session as `tix login` does. `tix login
--ssh-key PATH` SHALL use only that private key, asking for its passphrase on the terminal without echo
when it is encrypted.

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

### Requirement: An SSH key login takes no password and checks the server's namespace

`--ssh-key` SHALL NOT be combined with `--email` or `--password`. The CLI SHALL compute the namespace
from the server address it was given and SHALL refuse to sign when the server announces a different
one. A second factor SHALL be asked for exactly as after a password.

#### Scenario: A key login with an email is a usage error

- **WHEN** `tix login --ssh-key --email ada@example.com` runs
- **THEN** it exits 2 as a usage error

### Requirement: Logging in does not need a credential first

`tix login` SHALL reach a remote server without first confirming a credential, since it exists to obtain
one.

#### Scenario: A remote login with no token configured

- **WHEN** `tix --server URL login --ssh-key KEY` runs with no token
- **THEN** it signs in rather than failing on the missing credential

### Requirement: The command line sets a lease when it adds a task

`tix task add --lease D` SHALL create the task with lease D. A lease above the effective maximum SHALL exit 2 and
create nothing, as `tix task edit --lease` does.

#### Scenario: Adding a task with a lease

- **WHEN** `tix task add "build" --lease 2h` runs and the task is claimed without `--ttl`
- **THEN** the claim lasts 2h and reports the task as its source

#### Scenario: Adding a task with a lease above the maximum

- **WHEN** `tix task add "build" --lease 48h` runs where the maximum is 24h
- **THEN** it exits 2 and no task is created

### Requirement: Source settings from the command line

`tix sync source add` SHALL accept `--url`, `--project`, `--query`, `--file`, `--page-size` and `--mapping`,
store what is given on the source, and `tix sync source ls` SHALL show the stored settings and never a
credential.

#### Scenario: Adding a jira source with its url

- **WHEN** `tix sync source add jira platform --url https://jira.example --project PLAT` runs
- **THEN** `tix sync source ls -o json` shows the url and project on that source

### Requirement: The indexed flag is a deprecated no-op

`tix field put` SHALL accept `--indexed` without effect, hidden from help, and print a deprecation notice.

#### Scenario: An old script

- **WHEN** `tix field put rv severity --type string --indexed` runs
- **THEN** the field is saved and stderr says the flag is deprecated

### Requirement: Quick add on the command line reads dates in the configured order

`tix task add --parse` SHALL read a `due:` date in the order `output.time_format` writes dates, as the
browser and the TUI do. `--due` SHALL keep reading `YYYY-MM-DD` or RFC 3339 only, so a script means the
same date on every machine, and its help SHALL show an example of each.

#### Scenario: A dotted due date in a parsed title

- **GIVEN** `output.time_format: dotted`
- **WHEN** `tix task add --parse 'pay rent due:06.11.2026'` runs
- **THEN** the task falls due on 6 November 2026

### Requirement: Global admin commands

The CLI SHALL offer `tix global-admin grant EMAIL`, `revoke EMAIL` and `ls`, and `tix tenant user ls`,
`create`, `edit` and `rm`, each naming the tenant first.

#### Scenario: Revoking the last one

- **WHEN** `tix global-admin revoke` names the only global admin
- **THEN** it exits 6

### Requirement: Tenant create names its first admins

`tix tenant create KEY [NAME]` SHALL take the name as an argument or `--name`, not both, and a repeatable
`--admin EMAIL`. Generated passwords SHALL be printed once: as a table after a warning on stderr, or in
full in `-o json`.

#### Scenario: Two admins

- **WHEN** `tix tenant create test --name Test --admin a@x.test --admin b@x.test` runs
- **THEN** stdout carries both addresses with their passwords and stderr says they are shown once

### Requirement: tix whoami says where commands go

`tix whoami` SHALL report the target as a server URL or a local database path, the context that chose it, the
tenant, the user, and whether a token is presented and which layer supplied it, never its value. `tix status`
SHALL print the same lines before its report.

#### Scenario: Pointed at a server through a context

- **WHEN** `tix whoami` runs under a context naming a server and a token
- **THEN** it prints the server URL, the context name, the user the token belongs to, and that the token came
  from that context

#### Scenario: Nothing configured beyond the local opt-in

- **WHEN** `tix whoami` runs against the default database
- **THEN** it prints `local database` and the path, and says no server is configured

### Requirement: A missing token is explained

When a server refuses a command as unauthenticated and the command presented no token, the error SHALL name the
server and the `tix ctx add ... --token` command that supplies one.

#### Scenario: A server URL with no token

- **WHEN** `tix --server URL task show REF` runs with no token anywhere
- **THEN** it exits 5 with a message naming URL and `tix ctx add`

### Requirement: Password change and invite commands

`tix user create` and `tix user edit` SHALL take `--require-password-change`. `tix user invite
EMAIL [--role R] [--expires D]` SHALL print the link, and `tix user invite ls`, `revoke ID` and
`reissue ID` SHALL manage pending invites. `tix login` on a flagged account SHALL ask for the new
password twice on a terminal, and without one SHALL fail naming the web sign-in page.

#### Scenario: Login with no terminal

- **GIVEN** a flagged account
- **WHEN** `tix login --password -` runs with the password piped in
- **THEN** it fails saying a password change is required and where to make it, and prints no session

### Requirement: tix user time

`tix user time` SHALL show each level of the caller's date format and timezone with the values in force,
and `--format` and `--timezone` SHALL set the caller's own, `""` returning a field to the tenant default.
`tix tenant edit` SHALL take `--time-format` and `--timezone` for the tenant default.

#### Scenario: Set and inherit

- **WHEN** a person runs `tix user time --timezone Asia/Tokyo` and then `tix user time --timezone ""`
- **THEN** the second run shows the zone in use coming from the tenant or the deployment

### Requirement: Command line tables follow the account

Table output SHALL write timestamps in the caller's resolved format and zone. A format or zone set on the
machine (a flag, `TIX_OUTPUT_*`, or the configuration file) SHALL win for that field on that machine. A
struct a table has no layout for SHALL write its instants through the same style, never Go's default.

#### Scenario: Unset on the machine, set on the account

- **GIVEN** no `output.time_format` on the machine and an account format of `dotted`
- **WHEN** the person runs `tix task ls`
- **THEN** UPDATED reads like `06.11.2026 14:05`

### Requirement: tix audit ls shows and filters the channel

tix audit ls SHALL take repeatable --via and --token flags, SHALL accept via: and token: in --filter, SHALL exit with the usage code for an unknown channel, and its table SHALL carry a VIA column.

#### Scenario: Filtering by channel

- **WHEN** tix audit ls --via ssh runs on a store where nothing was done over SSH
- **THEN** it prints no entries

#### Scenario: The table names the channel

- **WHEN** tix audit ls prints a table
- **THEN** each row's VIA column names its channel and token or job
