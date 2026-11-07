## MODIFIED Requirements

### Requirement: Credentials are supplied externally and never persisted in outputs

Credentials for external systems SHALL be supplied through the server's environment or stored on the source,
sealed with the server's key. They SHALL NOT appear in exported snapshots, audit entries, event payloads, API
responses, screens, or logs, including error output. A response MAY name which credentials are stored and the
last four characters of one at least 12 characters long.

#### Scenario: Credentials absent from a snapshot

- **WHEN** a tenant with configured external sources is exported
- **THEN** the snapshot contains the source configuration without its credentials

#### Scenario: Credentials absent from audit entries

- **WHEN** an external source configuration is changed
- **THEN** the audit entry records the change without the credential value

#### Scenario: Credentials absent from error output

- **WHEN** an import fails because the external system rejected authentication
- **THEN** the error reports the rejection without including the credential

#### Scenario: Stored credentials are write-only

- **WHEN** a source is saved with an API key and then listed over the API, the CLI and the web screen
- **THEN** each shows that a password is stored and its last four characters, and none shows the key

## ADDED Requirements

### Requirement: Stored credentials are sealed

A credential stored on a source SHALL be encrypted with the server's sealing key, bound to its tenant, system and
source name. Without a key the server SHALL refuse to store one, naming the key and the environment variable to
use instead.

#### Scenario: No sealing key

- **WHEN** a credential is sent to a server with no `auth.totp_key`
- **THEN** it is refused with exit 6, naming `auth.totp_key` and `TIX_SYNC_<SOURCE>_TOKEN`, and nothing is stored

### Requirement: Stored values win over the environment

At run time a value stored on the source SHALL win over its `TIX_SYNC_<SOURCE>_*` variable. Stored credentials
SHALL win as a set, never mixed with credentials from the environment. A stored mapping document SHALL win over
`mapping_path`, which SHALL win over `TIX_SYNC_<SOURCE>_MAPPING`.

#### Scenario: Stored token and environment token

- **WHEN** a source stores a token and the server's environment also sets one
- **THEN** a run authenticates with the stored token

#### Scenario: Nothing stored

- **WHEN** a source stores no credentials and the environment sets a token
- **THEN** a run authenticates with the environment's token

### Requirement: Mapping document stored on the source

A source SHALL accept its mapping document itself, from the web dialog, the API or `tix sync source set
--mapping-file`, validate it when saved, and keep it across updates that do not send one until it is cleared.

#### Scenario: Uploaded mapping used by a run

- **WHEN** a mapping document is uploaded to a source whose mapping file is not readable on the server
- **THEN** a run reads the uploaded document and imports through it

#### Scenario: Invalid mapping refused at save

- **WHEN** a mapping document that names no project is uploaded
- **THEN** it is refused on the mapping field and nothing is stored

### Requirement: The server says what a source will use and lacks

Every listed source SHALL carry where its mapping, credentials and settings come from on the server, and what is
missing there. A run of a source missing anything SHALL be refused before any record is read, with one message
naming every missing piece and how to supply it.

#### Scenario: Nothing configured on the server

- **WHEN** an OpenProject source with no url, mapping or credentials is run
- **THEN** it is refused with exit 2, the message says it is missing on the server and names the mapping, the credentials and the url, and no adapter is built

### Requirement: Checking a source writes nothing

A source submitted with `check` SHALL be validated against the project and workflow its mapping names and
described, and nothing SHALL be stored.

#### Scenario: Check in the web dialog

- **WHEN** the source dialog is submitted with Check and an uploaded mapping
- **THEN** the dialog shows the target project, the mapped fields and statuses and what is missing, keeps the mapping in the form, and no source is saved
