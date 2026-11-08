## ADDED Requirements

### Requirement: A shared server reads no file a tenant names outside its directory

On `tix serve` a source's stored `file` and `mapping_path` SHALL be read only from inside
`<sync.file_root>/<tenant key>/`, with symbolic links resolved and required to stay inside. Without
`sync.file_root` a shared server SHALL refuse to store either setting, naming the field.

#### Scenario: An absolute path to a server file

- **WHEN** a tenant admin on `tix serve` saves a generic source whose `file` is `/srv/keys/account.json`
- **THEN** the save fails as invalid on `file`, and no run imports that file

#### Scenario: A link out of the tenant directory

- **WHEN** the tenant directory holds `rows.csv` as a link to a file outside it and a source names it
- **THEN** the source reports the file missing and a run is refused without reading it

#### Scenario: A file in the tenant directory

- **WHEN** `sync.file_root` is `/srv/tix-sync` and a source in tenant acme stores `file: rows.csv`
- **THEN** a run reads `/srv/tix-sync/acme/rows.csv`

### Requirement: A refused server path discloses nothing

A refusal of a stored path SHALL read the same whether the file exists, lies outside the tenant
directory, is a link out of it or cannot be opened, and SHALL NOT quote the file's content.

#### Scenario: Probing for a file

- **WHEN** a tenant admin saves `mapping_path: /etc/passwd` and then `mapping_path: /nonexistent`
- **THEN** both are refused with the same message

### Requirement: The server environment applies only to bound sources

On `tix serve` the `TIX_SYNC_<SOURCE>_*` variables SHALL apply only to sources `sync.env_sources` binds,
each entry `TENANT/SOURCE` or `TENANT/*`. An unbound source SHALL read none of them, and nothing it is
shown SHALL depend on which of them are set.

#### Scenario: Same name in another tenant

- **GIVEN** the server sets `TIX_SYNC_CORP_TOKEN` and binds only `default/corp`
- **WHEN** tenant acme adds a jira source named corp and lists it
- **THEN** its credentials read missing, no variable is named, and a run sends no token

### Requirement: Environment credentials go only to the environment's url

Credentials read from the environment SHALL be sent only to the url `TIX_SYNC_<SOURCE>_URL` names. A
source that stores a different url SHALL be reported missing credentials, told to store its own.

#### Scenario: A stored url beside environment credentials

- **WHEN** a bound source stores `url: https://elsewhere.test` and the environment sets a token and another url
- **THEN** the run is refused and `https://elsewhere.test` receives no request carrying the token

### Requirement: A direct command is the operator's own

A command run against the database directly SHALL read any file and any `TIX_SYNC_<SOURCE>_*` variable a
source names, as the operator's own process, subject to the url rule for environment credentials.

#### Scenario: A mapping path on a direct command

- **WHEN** `tix sync source add generic ops --mapping /home/me/ops.yaml` runs against a local database
- **THEN** the path is stored and a run reads it

## MODIFIED Requirements

### Requirement: Stored settings win over the environment

At run time a setting stored on the source SHALL be used in place of its `TIX_SYNC_<SOURCE>_*` variable, and
a setting the source does not store SHALL be read from that variable when the environment applies to the
source.

#### Scenario: Stored beats the environment

- **WHEN** a source stores `file` and `TIX_SYNC_<SOURCE>_FILE` names another file
- **THEN** the run reads the stored file

#### Scenario: The environment fills the gaps

- **WHEN** a bound source stores no mapping path and `TIX_SYNC_<SOURCE>_MAPPING` names one
- **THEN** the run uses the mapping the variable names

### Requirement: Stored credentials are sealed

A credential stored on a source SHALL be encrypted with the server's sealing key, bound to its tenant, system and
source name. Without a key the server SHALL refuse to store one, naming the key, and naming the environment
variable to use instead only when the environment applies to the source.

#### Scenario: No sealing key

- **WHEN** a credential is sent to a direct command with no `auth.totp_key`
- **THEN** it is refused with exit 6, naming `auth.totp_key` and `TIX_SYNC_<SOURCE>_TOKEN`, and nothing is stored

### Requirement: Stored values win over the environment

At run time a value stored on the source SHALL win over its `TIX_SYNC_<SOURCE>_*` variable. Stored credentials
SHALL win as a set, never mixed with credentials from the environment. A stored mapping document SHALL win over
`mapping_path`, which SHALL win over `TIX_SYNC_<SOURCE>_MAPPING`.

#### Scenario: Stored token and environment token

- **WHEN** a source stores a token and the server's environment also sets one
- **THEN** a run authenticates with the stored token

#### Scenario: Nothing stored

- **WHEN** a bound source stores no credentials or url and the environment sets a token and a url
- **THEN** a run authenticates with the environment's token against the environment's url
