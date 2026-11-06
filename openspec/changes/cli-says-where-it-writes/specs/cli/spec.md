## ADDED Requirements

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
