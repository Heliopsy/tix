# What's new, from the changelog the binary was built with

## Why

The owner asked: "when I click the version can we open a page with changes, just so we can see the
latest". The version in the sidebar footer is plain text unless a newer release exists, and the
release screen only shows the notes of the newest published release, fetched from GitHub. An
installation that is up to date, or one with no route out, has no way to see what its own version
changed.

## What Changes

- `CHANGELOG.md`, which release-please writes, is embedded into the binary by package `main` and
  handed to a new `internal/changelog` package that parses it into entries.
- The running version in the sidebar footer becomes a link to `/release#changes`.
- The release screen gains a "What's new" section: the running version's entry first, marked as the
  one running here, then the ten entries before it, each folded in a `<details>`. A build that is not a
  tagged release (`dev`, `0.31.0+dirty`, a fork's version) says so and lists the newest entries.
- Entries render through `internal/markdown` with links stripped, the same policy the feed notes use.
- The feed's notes are shown only when a newer release exists; when the installation is current they
  repeat the running version's entry.

## Impact

- `main.go`, new `internal/changelog`, `internal/web/release.go`, `internal/web/session.go`,
  `internal/web/templates/release.html`, `internal/web/templates/partials.html`,
  `internal/web/assets/app.css`, `internal/i18n/bg_release.go`, `docs/web-ui.md`.
- No new route, service method, CLI flag or API change.
