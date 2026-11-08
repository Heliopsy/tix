# Design

## Where the option lives

The mode is an option of `sqlite.Open` (`WithJournalMode`), carried by `connect.Overrides.JournalMode`
from `embedded.Start`. Nothing reads it from configuration: the embedded package reads no configuration at
all, and a server operator has no reason to leave WAL, which is what lets readers run beside the writer.
An empty value means WAL everywhere, so every existing caller is unchanged without touching it.

Only `wal` and `delete` are accepted. `truncate` and `persist` behave like `delete` for this purpose with
more edge cases, `memory` and `off` give up crash safety, and a list of two is easy to test and document.

## Durability in delete mode

SQLite documents that `synchronous=NORMAL` is safe from corruption in WAL mode, but in rollback-journal
mode a power loss at the wrong moment can corrupt the database. The point of `delete` is a file that is
whole on disk after every commit, so it runs with `synchronous=FULL`. The extra fsync per commit is cheap
on a phone's single-user workload.

## Readers in delete mode

The store keeps its separate reader pool. In rollback-journal mode a reader blocks the writer from
committing and the writer's exclusive lock blocks readers; the existing busy timeout (5 s) covers both, and
an embedded instance serves one app. Collapsing the pools for this mode would fork the store's
transaction code for no measured gain.

## Checkpoint at stop

`Store.Checkpoint` runs `PRAGMA wal_checkpoint(TRUNCATE)` on the writer connection and does nothing in
delete mode. `embedded.Instance.Stop` calls it after the server and workers have stopped and before the
connection closes, through an interface assertion on the store, so `internal/connect` and the PostgreSQL
store are untouched. A checkpoint that cannot complete (a reader in another process holding the file) is
reported by SQLite as busy rather than an error; Stop reports it as a conflict error but still closes the
database, since a stop that leaves the file open is worse than a `-wal` that the next open replays.
`tix serve` does not checkpoint on shutdown: SQLite's own close checkpoints when it is the last connection,
and a server's database is not expected to be copied mid-flight.
