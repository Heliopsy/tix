# Tasks

## Store

- [ ] `sqlite.WithJournalMode`, mode-dependent DSN pragmas and open-time check
- [ ] `Store.Checkpoint` running `wal_checkpoint(TRUNCATE)` in WAL mode
- [ ] Tests on a temporary database for both modes, an unknown mode and the checkpoint

## Embedding

- [ ] `connect.Overrides.JournalMode` passed to `sqlite.Open`
- [ ] `embedded.Options.JournalMode`, validated before opening
- [ ] Checkpoint in `Instance.Stop` before closing
- [ ] `journal_mode` in the `TixStart` options
- [ ] Tests: delete mode leaves no `-wal`, WAL stop leaves an empty `-wal`, unknown mode is invalid

## Docs

- [ ] `docs/embedding.md`: the option, the C ABI table and syncing a database folder
- [ ] `docs/tenancy.md` and `.golangci.yml`: the checkpoint pragma beside the journal mode one
