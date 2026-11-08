## ADDED Requirements

### Requirement: A transfer view exports and imports by file path

The local terminal interface SHALL offer a transfer view that exports a project's tasks, a bundle or
a snapshot to a file path, and imports each from a file path, through the same service calls as the
CLI.

#### Scenario: Exporting a project's tasks

- **WHEN** a person opens the transfer view in `tix tui`, picks a task export of INFRA and names
  `infra.csv`
- **THEN** the file holds the same CSV `tix export tasks -p INFRA` writes

### Requirement: A task import in the terminal walks through the column mapping

A task import in the transfer view SHALL first run as a dry run, show which column fills which field
and the rows in error, and write only after the person confirms that mapping.

#### Scenario: Confirming a suggested mapping

- **WHEN** a person imports `jira.csv` into INFRA
- **THEN** the view shows the suggested columns and preview rows, and nothing is written until the
  person confirms

### Requirement: The hosted transfer view names the SSH command

Over SSH the server cannot read or write the client's files, so the transfer view SHALL name the
`ssh` command that performs each transfer through standard input or output instead of asking for a
path, and the registry SHALL record this as a TUI limitation rather than a gap.

#### Scenario: Opening the transfer view over SSH

- **WHEN** a person opens the transfer view in an SSH session as `acme`
- **THEN** the view shows commands such as `ssh acme@host export tasks -p KEY > FILE` and asks for no path
