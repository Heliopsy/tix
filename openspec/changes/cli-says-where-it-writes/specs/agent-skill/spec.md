## ADDED Requirements

### Requirement: The skill checks the target before any work

The skill SHALL tell an agent to run `tix whoami` before any other tix command, to stop when it reports a local
database while a server is expected, and SHALL give the `tix ctx add NAME --server URL --token TOKEN --use`
line that sets a server up with a token every command presents.

#### Scenario: An agent on a machine with only a local database

- **WHEN** an agent following the skill is asked to work on a server and `tix whoami` reports a local database
- **THEN** the skill directs it to stop and set up the context before writing anything
