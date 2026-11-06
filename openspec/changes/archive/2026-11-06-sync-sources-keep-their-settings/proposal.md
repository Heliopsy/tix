# Sync sources keep their settings, and the indexed flag goes

## Why

A sync source took a `config` map and a `mapping_path`, checked the map for credential keys and then dropped
both. Every setting came from `TIX_SYNC_<SOURCE>_*` at run time, so a caller who sent config got a success
with no effect, and settings could only differ between sources through their names. The columns to hold them
have existed since the first schema and were never written.

Custom field definitions carry an `indexed` flag that is stored, shown and carried through bundles, and that
nothing reads: no index is built and no query plans differently. A setting that changes nothing misleads
whoever sets it.

## What Changes

- A sync source stores its non-secret settings: `url`, `project`, `query`, `file` and `page_size` in
  `config`, plus `mapping_path`. Keys are validated per system and an unknown key is refused. Credential
  keys are still refused and still come only from the environment.
- At run time a stored value wins over the matching `TIX_SYNC_<SOURCE>_*` variable, and the variable fills
  anything the source leaves empty, so an environment-only setup keeps working unchanged.
- The stored settings appear on every surface that shows a source: the HTTP response, `tix sync source ls`,
  and the web Sources table. `tix sync source add` gains `--url`, `--project`, `--query`, `--file`,
  `--page-size` and `--mapping`; the web form gains the same fields.
- The `indexed` flag is removed from field definitions on the API, CLI, TUI, web, bundles, docs, skill and
  demo. Input that still sends it (API bodies, old bundles, old snapshots) is accepted and the key ignored.
  `tix field put --indexed` stays as a hidden, deprecated no-op so scripts do not break. Migration 0029
  drops the column on both engines.

## Capabilities

### Modified Capabilities

- `external-sync`: stored settings, their validation and their precedence over the environment.
- `workflows`: the indexed custom field requirement is removed.
- `cli`: the source flags, the deprecated `--indexed`.
- `http-api`: the source body and response carry the settings; `indexed` is ignored on input.
- `web-ui`: the source form and table show the settings.
