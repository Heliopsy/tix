# Tasks

- [x] `internal/selfupdate`: install-method detection (release archive, go install, managed, unwritable)
- [x] Release lookup and per-platform archive naming, matching what GoReleaser publishes
- [x] Verified download: checksum parsed from the release's checksum file, refusal on mismatch or absence
- [x] Atomic replacement: temp file beside the target, rename over it; the Windows move-aside path
- [x] `cmd/update.go`: `--check`, `--version`, refusals before download
- [x] Tests over an httptest server serving a real archive and checksum file
- [x] Failure tests: bad checksum, unlisted archive, truncated body, unwritable directory
- [x] `docs/upgrading.md`, linked from the index, honest about what the checksum proves
- [x] `ROADMAP.md`: move self-update into what shipped
- [x] `just ci` green, archive the change
