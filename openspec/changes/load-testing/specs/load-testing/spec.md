## ADDED Requirements

### Requirement: A local load run measures a fresh scratch server

`just load` SHALL seed a scratch database with several tenants, people with passwords and tokens, projects
and tasks with Markdown descriptions, comments, tags and assignees, then serve it from a process started
after seeding, drive the configured users at it for the configured duration, and stop that process by its
own PID.

#### Scenario: Default run

- **WHEN** `just load` runs with no arguments
- **THEN** four users (two browsers, one phone, one agent) run for two minutes against a newly started server
- **AND** no `tix serve` process the run started is left running afterwards

### Requirement: The report gives latency, errors and server memory

A run SHALL report, per route, the request count, p50, p95, p99 and maximum latency, the count of each status
and of errors, and for a local run the server's start, steady-state and peak resident memory, where peak is
the largest of every sampled VmRSS and the final VmHWM.

#### Scenario: A spike between two samples

- **GIVEN** samples whose largest VmRSS is 200 MiB and a final VmHWM of 210 MiB
- **THEN** the reported peak is 210 MiB

#### Scenario: A refused transition is not an error

- **WHEN** a request answers 409
- **THEN** it is counted under its status and not as an error, while a 5xx or a request with no answer is

### Requirement: A run fails over its memory budget

A run SHALL exit with status 1 when the server's peak resident memory exceeds `--max-rss`. The default
budget SHALL be 128 MiB for runs of up to four users and none above that, and `--max-rss 0` SHALL disable it.

#### Scenario: Over budget

- **WHEN** a four-user run measures a peak of 129 MiB with the default budget
- **THEN** the report says FAIL and the exit status is 1

#### Scenario: Memory that cannot be read

- **WHEN** the server is remote
- **THEN** the budget is reported as not checked and does not fail the run

### Requirement: A remote run is read-only unless writes are allowed

A run with `--target` SHALL send no mutating request other than signing in unless `--allow-writes` is given,
and SHALL print a warning banner naming the target and whether writes are on before its first request.

#### Scenario: No --allow-writes

- **WHEN** an agent user on a remote run without `--allow-writes` would create a task
- **THEN** the request is not sent and is counted as a refused write

#### Scenario: A project named without --allow-writes

- **WHEN** a remote run is given `--project` without `--allow-writes`
- **THEN** the run is refused before any request

### Requirement: Remote writes stay in a scratch project

With `--allow-writes`, a remote run SHALL create tasks only in the project named by `--project`, created when
missing, or in a newly created project with a generated key, and SHALL change only tasks it created. A
generated key that names an existing project SHALL be refused.

#### Scenario: Generated key collides

- **WHEN** the generated scratch key names a project that already exists
- **THEN** the run refuses to start and writes nothing

### Requirement: Load runs nightly in CI

A scheduled and manually dispatchable workflow SHALL run the four-user profile for two minutes with the
128 MiB budget and upload the report as an artifact. It SHALL NOT run on pull requests.

#### Scenario: Nightly run

- **WHEN** the nightly schedule fires
- **THEN** the report appears in the job summary and as the `load-report` artifact
