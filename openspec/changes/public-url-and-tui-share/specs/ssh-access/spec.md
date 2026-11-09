## ADDED Requirements

### Requirement: Task link runs over SSH

`tix task link` SHALL be hosted over SSH wherever `tix task show` is, and SHALL print the task's
`web_url`. `--base` SHALL be refused there with exit 2, like the other inputs that name something on
the serving machine. Without `server.public_url` on the server it SHALL exit 2 naming the key.

#### Scenario: The server knows its address

- **GIVEN** a server with `server.public_url: https://tix.example.com`
- **WHEN** `ssh tix.example.com task link infra-12 --as markdown` is run
- **THEN** it prints `[INFRA-12 Title](https://tix.example.com/tasks/infra-12)`

#### Scenario: The server does not

- **GIVEN** a server without `server.public_url`
- **WHEN** `ssh HOST task link infra-12` is run
- **THEN** it exits 2 and names `server.public_url`
