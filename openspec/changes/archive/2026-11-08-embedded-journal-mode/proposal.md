# Journal mode for an embedded database

## Why

tix-mobile keeps its database wherever the user points it, and a common choice is a folder Nextcloud,
Syncthing or iCloud Drive mirrors to other devices. In write-ahead logging mode a commit lands in
`tix.db-wal` and reaches `tix.db` only at the next checkpoint, so the copy a sync tool picks up between
checkpoints is the main file without its latest commits, or a main file and a `-wal` file uploaded at
different moments. The embedded store hard-codes WAL and refuses to open in any other mode, so the app has
no way to ask for a single file that is whole after every commit.

## What Changes

- `embedded.Options.JournalMode` takes `wal` (the default when empty) or `delete`. Anything else is an
  invalid error before the database is opened.
- `TixStart` accepts `journal_mode` with the same values.
- The SQLite store gains an open option for the journal mode. `delete` mode runs with `synchronous=FULL`,
  since NORMAL in rollback-journal mode can lose a commit on power loss; WAL keeps `synchronous=NORMAL`.
  The open-time check confirms the mode that was asked for rather than always WAL.
- Stopping an embedded instance runs `PRAGMA wal_checkpoint(TRUNCATE)` before closing the database, so a WAL
  database is left as one complete file with an empty `-wal`.
- **Not changed:** `tix serve`, the CLI and every other caller open SQLite in WAL as before. There is no
  configuration key or flag for it.
