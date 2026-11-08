## ADDED Requirements

### Requirement: The CLI prints a task's web address

`tix task link REF` SHALL print the absolute address of the task's screen in the browser interface:
the server URL of the context it talks to, or `--base`, followed by `/tasks/REF`. `--as markdown`
SHALL print the Markdown link and `--as text` the reference and title. A structured format SHALL
carry all three. An unknown reference SHALL exit 3.

#### Scenario: A remote context

- **GIVEN** a context whose server is `https://tix.example.com`
- **WHEN** `tix task link infra-12` is run
- **THEN** it prints `https://tix.example.com/tasks/infra-12`

#### Scenario: A local database names no address

- **GIVEN** a context that opens a local database
- **WHEN** `tix task link infra-12` is run without `--base`
- **THEN** it exits 2 and says to pass `--base`
