# Tasks

## Store

- [x] `sqlite.WithJournalMode`, mode-dependent DSN pragmas and open-time check
- [x] `Store.Checkpoint` running `wal_checkpoint(TRUNCATE)` in WAL mode
- [x] Tests on a temporary database for both modes, an unknown mode and the checkpoint

## Embedding

- [x] `connect.Overrides.JournalMode` passed to `sqlite.Open`
- [x] `embedded.Options.JournalMode`, validated before opening
- [x] Checkpoint in `Instance.Stop` before closing
- [x] `journal_mode` in the `TixStart` options
- [x] Tests: delete mode leaves no `-wal`, WAL stop leaves an empty `-wal`, unknown mode is invalid

## Docs

- [x] `docs/embedding.md`: the option, the C ABI table and syncing a database folder
- [x] `docs/tenancy.md` and `.golangci.yml`: the checkpoint pragma beside the journal mode one
