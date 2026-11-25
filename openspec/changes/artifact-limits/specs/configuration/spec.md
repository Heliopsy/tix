## ADDED Requirements

### Requirement: Artifact limit keys

The configuration SHALL define `artifacts.max_bytes`, defaulting to 1 MiB, and `artifacts.allowed_types`,
defaulting to common images, PDF, plain text, Markdown, CSV, JSON, YAML, archives and application/octet-stream.
`tix serve` SHALL take `--artifact-max-bytes` and `--artifact-allowed-types`, which win only when typed. A
non-positive size or a malformed type SHALL be refused naming the key.

#### Scenario: Defaults change nothing

- **WHEN** no artifact key is configured
- **THEN** an artifact blob up to 1 MiB of an accepted type is stored as before

#### Scenario: Malformed type

- **WHEN** `artifacts.allowed_types` contains `text`
- **THEN** loading the configuration fails naming `artifacts.allowed_types`

### Requirement: Artifact uploads are not held to the body limit

The artifact upload routes SHALL accept a request body large enough to carry a blob of `artifacts.max_bytes` once
encoded, whatever `--max-body-bytes` is. A body past that SHALL be refused with 413 naming `artifacts.max_bytes`.

#### Scenario: Small general body limit

- **WHEN** `--max-body-bytes` is 4 KiB and a 512 KiB artifact is put under a 1 MiB artifact limit
- **THEN** the artifact is stored
