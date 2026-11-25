## ADDED Requirements

### Requirement: The embedded build leaves out what an in-process API does not need

The library SHALL be built with the `tix_embedded` build tag. A tagged build SHALL serve the HTTP API and
event stream for a SQLite database, and SHALL NOT link the browser interface, the PostgreSQL store or
terminal table rendering. The `tix` binary SHALL never be built with the tag.

#### Scenario: The browser interface is absent

- **GIVEN** an instance started through the embedding library
- **WHEN** a client requests `/` or `/assets/app.css`
- **THEN** the answer is 404 and `/api/v1/whoami` still answers for a valid token

#### Scenario: An excluded package is linked

- **WHEN** a change makes the tagged library build depend on `github.com/jackc/pgx`
- **THEN** the dependency guard test fails naming the package

### Requirement: Operations a build omits are declared

Every operation that a tagged build does not serve SHALL be marked in the capability registry, and the
HTTP contract tests SHALL pass for both the full and the tagged build.

#### Scenario: An omitted operation is not marked

- **WHEN** a tagged build stops serving an operation that the registry does not mark as omitted
- **THEN** the contract test for the tagged build fails naming the operation

### Requirement: Library size is reported

The library CI job SHALL record the size of each built library, full and tagged, in the job summary.

#### Scenario: A pull request touching the library

- **WHEN** CI builds the libraries for a pull request
- **THEN** the job summary lists each target with its size
