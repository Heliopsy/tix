# Replace this binary with the current release

## Why

There is no upgrade path from inside tix. Someone who installed with `install.sh` finds out a release
exists because the web interface tells them, and then has to remember which of five ways they installed it
and go and re-run that. The binary already knows its own version, its own platform and where it is on disk,
and the release already publishes an archive per platform and a checksum file beside them, so every piece
of the answer exists except the command.

## What Changes

- **`tix update`** replaces this binary with the current release, in place.
- **`tix update --check`** reports whether a newer release exists and exits without writing anything.
- **`--version`** installs a named release rather than the newest, which is also how to go backwards.
- **Refusals, before anything is downloaded.** A binary a package manager owns, or one `go install` built,
  is not this command's to replace: it says which command to run instead. A target directory that cannot
  be written says so before the download rather than after it.
- **The replacement is atomic.** The new binary is written beside the target and renamed over it, so an
  interrupted download can never leave a truncated file on someone's `PATH`.

## What this does not claim

The checksum is verified against `checksums.txt` from the same release. That catches a truncated or
corrupted download, and a mirror serving the wrong bytes. It is **not** a supply-chain guarantee: both the
archive and the checksum file come from the same host, so the real trust anchor is TLS to that host. The
releases are also signed with cosign, and verifying those signatures needs a verifier this binary does not
carry; `docs/` says so plainly rather than letting "checksum verified" read as more than it is.

Restarting a running server is deliberately not part of this. A tracker that restarts itself because it
noticed an open file handle is worse than one that says a restart is pending.

## Impact

- New `internal/selfupdate`, stdlib only: install-method detection, release lookup, verified download,
  atomic replacement.
- New `cmd/update.go`.
- `docs/` gains an upgrading page; `ROADMAP.md` moves this out of "planned".
- No new dependency, and no change to how releases are built: GoReleaser already publishes what this reads.
