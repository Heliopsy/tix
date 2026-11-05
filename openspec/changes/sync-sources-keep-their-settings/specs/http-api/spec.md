## ADDED Requirements

### Requirement: The source body and response carry the settings

`PUT /api/v1/sync/sources` SHALL store `config` and `mapping_path`, and its response and the source listing
SHALL return them without any credential.

#### Scenario: Round trip

- **WHEN** a source is put with config `url` and a mapping path
- **THEN** the response and `GET /api/v1/sync/sources` both return that url and mapping path
