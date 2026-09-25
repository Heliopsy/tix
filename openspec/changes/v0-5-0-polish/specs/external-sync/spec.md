## MODIFIED Requirements

### Requirement: A run records where it got to

An import run SHALL store its watermark only when the run advances past records it has committed, and a
run that fails SHALL leave the stored watermark at the last position it successfully committed.

A full refresh ignores the stored watermark for the duration of the run. It SHALL NOT overwrite it with an
empty value: a full refresh that fails before committing a page leaves the source exactly as it found it,
so a transient network error during a refresh does not turn the next incremental run into a full re-import.

The audit entry a run writes SHALL carry the watermark the run reached, because that entry is the durable
record of the run and the immediate command output is not.

#### Scenario: A full refresh that fails leaves the watermark alone

- **WHEN** a run started with a full refresh fails before any page is committed
- **THEN** the stored watermark is the one the previous run left, and the run is recorded as failed

#### Scenario: A full refresh that succeeds stores the new watermark

- **WHEN** a run started with a full refresh commits pages and completes
- **THEN** the stored watermark is the one the final page carried

#### Scenario: An incremental run that fails keeps its position

- **WHEN** a run fails after committing some pages
- **THEN** the stored watermark covers exactly the pages that were committed

#### Scenario: The audit entry carries the watermark

- **WHEN** a run completes and writes its audit entry
- **THEN** that entry records the watermark the run reached, not an empty value

### Requirement: A source that cannot be read is reported as an upstream failure

An import SHALL report a failure to read the external system as an upstream failure, naming the source
and how far the run got, and SHALL NOT report it as an internal error. This covers a system that could
not be reached, one that refused the credentials, and one that returned something the adapter could not
read.

The message SHALL NOT include anything the adapter was configured with, so that a url carrying a token
or a header cannot be re-rendered into an error someone pastes into an issue.

#### Scenario: An unreachable source

- **WHEN** the external system cannot be reached
- **THEN** the failure says the source could not be read, names the source and the number of records
  processed, and is classified as an upstream failure rather than an internal error

#### Scenario: The failure discloses no configuration

- **WHEN** the underlying cause carries a url, a header or a credential
- **THEN** none of it appears in the reported failure

### Requirement: A run's outcome is presented, not dumped

The command line SHALL render an import result as readable output in every format it supports, and SHALL
NOT fall back to printing the underlying structure.

#### Scenario: Table output

- **WHEN** a run completes and the output format is the default
- **THEN** the created, updated and skipped counts, the source, and the watermark are printed as a table

#### Scenario: A dry run says so

- **WHEN** a run is a dry run
- **THEN** the output states that nothing was written
