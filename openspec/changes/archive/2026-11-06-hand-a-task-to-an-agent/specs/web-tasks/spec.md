## ADDED Requirements

### Requirement: The task screen hands the task to a coding agent

The task screen's header, as a page and in the task dialog, SHALL carry a "Hand to an agent" button
that opens a dialog with a Close control that Escape also closes. The dialog SHALL show a prompt that
names the task's reference, its title and the server's address, and the tix commands that claim,
show, comment on and release the task.

#### Scenario: The dialog names this task on this server

- **GIVEN** task INFRA-12 titled "Fix the build" on a server reached at `https://tix.example.com`
- **WHEN** a reader opens the task and presses "Hand to an agent"
- **THEN** a dialog shows a prompt naming INFRA-12, "Fix the build" and `https://tix.example.com`
- **AND** `tix claim task INFRA-12` is part of it

### Requirement: Every command a hand-off names is real

Every `tix` command and flag the hand-off prompt names SHALL exist in the CLI, and every tool the MCP
variant names SHALL be one `tix mcp` exposes.

#### Scenario: A renamed command fails the build

- **GIVEN** the prompt names `tix claim task`
- **WHEN** that command is renamed in the CLI
- **THEN** the test that resolves the prompt's commands against the CLI fails

### Requirement: Each documented client gets its own way in

The dialog SHALL carry one tab per client documented in `docs/mcp.md`. A client with a command that
starts it with a prompt SHALL show that command; a client without one SHALL show the prompt to paste.
The dialog SHALL also offer the prompt phrased for an agent with the tix MCP tools.

#### Scenario: Claude Code starts on the task

- **WHEN** the reader picks the Claude Code tab
- **THEN** it shows `claude` followed by the prompt as one quoted argument

### Requirement: A hand-off is safe to paste into a shell

A command in the dialog SHALL pass the prompt as one argument that a POSIX shell does not expand,
whatever the title holds: quotes, `$()`, backticks, `$` or line breaks. No hand-off text SHALL carry
an API token.

#### Scenario: A hostile title stays text

- **GIVEN** a task titled `` it's $(rm -rf ~) and `whoami` ``
- **WHEN** its Claude Code command is run by `/bin/sh`
- **THEN** `claude` receives the prompt with the title exactly as written, folded to one line

### Requirement: The dialog works without scripting and remembers the client

Each block in the dialog SHALL have a Copy button. With scripting, the last client tab chosen SHALL
be remembered per browser; without scripting every client's block SHALL be drawn as plain text.

#### Scenario: The last tab comes back

- **GIVEN** a reader who picked the Codex tab
- **WHEN** they open the dialog on another task
- **THEN** the Codex tab is the one shown
