## ADDED Requirements

### Requirement: A source stores its non-secret settings

A sync source SHALL store `mapping_path` and a `config` map. The config keys SHALL be `file` and `page_size`
for generic, and `url`, `project`, `query` and `page_size` for jira and openproject. Any other key SHALL be
refused as invalid naming the keys the system accepts, and `page_size` SHALL be a positive integer.

#### Scenario: Settings are kept

- **WHEN** a jira source is saved with config `url` and `project` and a mapping path
- **THEN** reading the source back returns the same url, project and mapping path

#### Scenario: An unknown key is refused

- **WHEN** a generic source is saved with config key `url`
- **THEN** the save fails as invalid, nothing is stored, and the error names `file` and `page_size`

### Requirement: Credentials are refused in source settings

A source's config SHALL NOT carry a credential. A key naming a token, user, password or any other credential
SHALL be refused as invalid before any other check, and credentials SHALL be read from the environment only.

#### Scenario: A token in config

- **WHEN** a source is saved with config key `token`
- **THEN** the save fails as invalid, saying credentials are read from the environment

### Requirement: Stored settings win over the environment

At run time a setting stored on the source SHALL be used in place of its `TIX_SYNC_<SOURCE>_*` variable, and
a setting the source does not store SHALL be read from that variable as before.

#### Scenario: Stored beats the environment

- **WHEN** a source stores `file` and `TIX_SYNC_<SOURCE>_FILE` names another file
- **THEN** the run reads the stored file

#### Scenario: The environment fills the gaps

- **WHEN** a source stores no mapping path and `TIX_SYNC_<SOURCE>_MAPPING` names one
- **THEN** the run uses the mapping the variable names
