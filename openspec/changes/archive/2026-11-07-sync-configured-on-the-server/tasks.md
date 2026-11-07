# Tasks

## 1. Contract and stores

- [x] 1.1 `SyncSource.mapping`, `credentials`, `resolved`; sealed fields never serialized
- [x] 1.2 `SyncSourceInput.mapping`, `clear_mapping`, `token`, `user`, `password`, `clear_credentials`, `keep_settings`, `check`
- [x] 1.3 Migration 0034, both stores, rewind tests drop the new columns

## 2. Service

- [x] 2.1 Seal credentials with the server key, refuse without one, hints only for long values
- [x] 2.2 Keep mapping and credentials across updates, clear on request
- [x] 2.3 `resolved` on put and list; refuse an unready run naming everything missing
- [x] 2.4 `check` validates against the mapped project and workflow and writes nothing
- [x] 2.5 Audit entries carry credential names only

## 3. Surfaces

- [x] 3.1 Web: server note, per-source mapping/credentials/missing, dialog with mapping upload, credentials, Check, Set up
- [x] 3.2 CLI: `tix sync source set`, upload and credential flags, `ls` columns, missing on stderr
- [x] 3.3 `just openapi`
- [x] 3.4 Bulgarian strings

## 4. Docs

- [x] 4.1 `docs/migrating.md` rewritten around sync running on the server
- [x] 4.2 Skill section on copying a few tasks versus syncing a tracker
