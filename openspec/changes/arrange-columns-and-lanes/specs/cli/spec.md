## ADDED Requirements

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
