## ADDED Requirements

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

The CLI SHALL use exit code 0 for success, 1 for a generic error, 2 for a usage error, 3 for not found or no task available, 4 for conflict or lease expired, and 5 for an authentication or authorization failure. These codes SHALL be documented in help output.

#### Scenario: Success

- **WHEN** a command completes normally
- **THEN** it exits 0

#### Scenario: Usage error

- **WHEN** a command is invoked with an unknown flag or a missing required argument
- **THEN** it exits 2 and prints usage to standard error

#### Scenario: Empty queue

- **WHEN** a claim-next command finds no eligible task
- **THEN** it exits 3

#### Scenario: Lease expired

- **WHEN** a command presents an expired lease token
- **THEN** it exits 4

#### Scenario: Permission denied

- **WHEN** the actor is not permitted to perform the operation
- **THEN** it exits 5

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

The CLI SHALL provide the command groups `task`, `project`, `workflow`, `field`, `claim`, `comment`, `dep`, `label`, `user`, `token`, `ctx`, `config`, `doctor`, `serve`, `tui`, `export`, `import`, `sync`, `webhook`, `prune`, `docs`, `completion`, and `version`.

#### Scenario: Every group is reachable

- **WHEN** the root help is displayed
- **THEN** each of the listed command groups appears

#### Scenario: Unknown command

- **WHEN** an unrecognized command is invoked
- **THEN** the CLI exits 2 and suggests the closest matching command

#### Scenario: Version output

- **WHEN** `tix version` is run
- **THEN** the version is printed, and `-o json` yields it as structured data along with build metadata

### Requirement: Shell completion

The CLI SHALL generate completion scripts for bash, zsh, and fish, and those completions SHALL include dynamic completion of task references, project keys, labels, and statuses.

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
