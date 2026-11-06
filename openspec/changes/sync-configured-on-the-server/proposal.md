# Sync is configured where the user is, and runs on the server

## Why

A user on 0.43.0 configured an OpenProject source from their laptop, then found out the hard way that sync runs
on the server: the mapping had to be a file on the server's disk and the credentials had to be environment
variables in the server's process. Nothing they could reach let them supply either, and nothing told them. They
gave up and copied tasks with a script instead, which lost the assignees.

## What Changes

- A sync source stores its mapping document itself (`mapping`), uploaded from the web dialog, pasted, sent over
  the API, or uploaded by `tix sync source set NAME --mapping-file FILE`. `mapping_path` keeps working for
  mappings kept on the server. The stored document wins over `mapping_path`, which wins over
  `TIX_SYNC_<SOURCE>_MAPPING`.
- A sync source stores its credentials (`token`, `user`, `password`), sealed with the server's key
  (`auth.totp_key`, AES-GCM, bound to tenant, system and name). They are write-only: responses, screens, audit
  entries, events and logs carry only which ones are set and, for one of 12 characters or more, its last four.
  Without a key, storing one is refused (exit 6) naming the key and the variable to use instead. A stored set
  wins over the `TIX_SYNC_<SOURCE>_TOKEN`/`_USER`/`_PASSWORD` variables, as a whole.
- Mapping and credentials are kept when an update leaves them out, and removed by `clear_mapping` and
  `clear_credentials`. `keep_settings` keeps the stored settings too, for an update that only sets those.
- Every source reports `resolved`: where a run takes its mapping, credentials and each setting from on the
  server, and what is missing there. A run whose source is missing anything is refused before anything is
  read, with one message naming every missing piece and how to supply it.
- `check` validates a source against the project and workflow its mapping names, describes the mapping, and
  writes nothing. The web dialog's Check button uses it.
- Web: the sync screen says that runs execute on the server; each row says which mapping and credentials it
  uses and what the server is missing; the source dialog takes a mapping document (text or file upload),
  credentials and Check, and a Set up link reopens it filled in. CLI: `tix sync source set NAME`, and
  `--mapping-file`, `--token-stdin`, `--user`, `--password-stdin`, `--clear-mapping`, `--clear-credentials`,
  `--check` on `add` and `set`; `ls` gains mapping, credentials and readiness columns.
- Migration 0034 adds `mapping`, `credentials_sealed`, `credentials_nonce` and `credential_hints` to
  `sync_sources`.

## Impact

- `core.SyncSource`, `core.SyncSourceInput`, `PUT /api/v1/sync/sources`, `GET /api/v1/sync/sources`.
- `internal/service/sync.go`, `internal/service/syncsecrets.go`, both stores, the web sync screen, `cmd/sync.go`,
  `internal/output`, `docs/migrating.md`, the tix skill.
