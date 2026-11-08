## ADDED Requirements

### Requirement: An export holds no store connection while its client reads

A snapshot, task or bundle export SHALL be written to a spool inside its read transaction and sent to
the client only after the transaction has ended, keeping at most 8 MiB in memory and the rest in a
temporary file, so a client that reads slowly or not at all holds no store connection.

#### Scenario: Exports stalled on clients that do not read

- **GIVEN** more exports than the store has reader connections, each to a client that reads nothing
- **WHEN** another tenant lists its tasks
- **THEN** the listing succeeds

### Requirement: A tenant runs at most two exports at once

The service SHALL refuse a third concurrent export in one tenant with a conflict, and SHALL give the
slot back when an export ends, however it ends.

#### Scenario: A third export

- **GIVEN** two exports of tenant `acme` still running
- **WHEN** a third export of `acme` starts
- **THEN** it is refused with a conflict, and an export of another tenant is not
