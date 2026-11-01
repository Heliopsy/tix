## ADDED Requirements

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
