## ADDED Requirements

### Requirement: Task link prefers the server's web address

`tix task link` SHALL build the address from `--base` when given, else the task's `web_url`, else
the server URL of a remote context. A local database with `server.public_url` configured SHALL
therefore need no `--base`.

#### Scenario: The server names its public address

- **GIVEN** a context whose server is `http://10.0.0.5:8080` and a server with
  `server.public_url: https://tix.example.com`
- **WHEN** `tix task link infra-12` is run
- **THEN** it prints `https://tix.example.com/tasks/infra-12`
