# Design

## A section of /release, not its own route

The release screen from #149 already answers "what am I running and is there something newer". What
changed in the running version is the other half of the same question, and a separate
`/release/changes` page would have to repeat the status card to keep the update offer above it. So
the changelog is a section of `/release` with `id="changes"`, and the footer's version links to
`/release#changes`. The update card, the upgrade commands and the newest release's notes stay above
it; the fragment scrolls the reader to the part they clicked for, with or without scripting.

## Embedding

`go:embed` cannot reach a parent directory, and the repository root is package `main`, which nothing
can import. `main.go` embeds `CHANGELOG.md` and assigns it to `changelog.Source` before running the
command, the same way the linker assigns `version.Version`. That is wiring, not logic: parsing lives
in `internal/changelog`, and `internal/web` reads it. A binary or a test without the assignment has an
empty changelog, and the section says the build carries none.

## Matching the running version

`version.Version` is `dev` for an untagged build, a plain `major.minor.patch` for a release, and can be
`0.31.0+dirty` for a module build from an edited tagged tree. Only a plain `major.minor.patch` with an
entry of the same version is treated as a tagged release and highlighted. Anything else lists the
newest entries and says the build is not a tagged release, because highlighting `0.31.0` for a build
with local changes would claim those notes describe it.

## Links

The feed notes strip every link (`stripLinks`), and `TestPagesReferenceNoExternalOrigin` forbids an
external origin on any page. The changelog is ours, but its links point at GitHub, so the same
`stripLinks` runs on every entry: link text stays, targets and trailing commit hashes go.
