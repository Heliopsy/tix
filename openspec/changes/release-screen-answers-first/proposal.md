# The release screen answers first

## Why

The owner's verdict on `/release` was "this page is ugly". Screenshots at 1440px light and dark and at
390px, with an update on offer, with none, and with the feed unreachable, show why.

- The status was spread over a two-row settings list and a separate grey note under it, each a
  different width, so the one question the page is opened for ("is there something newer?") had no
  single answer to read.
- An install that was current was never told so: both rows just printed a version. An install that
  could not reach the feed and one that had not asked yet both read "not known".
- The release notes were a monospace `<pre>` showing raw Markdown (`##`, `**web:**`, backticks),
  wider than everything above it.
- The only guidance on an update was "Upgrading is the operator's business: this page only reports",
  which left the reader to go and find the commands.

## What Changes

- One status card at the top: a badge and a headline for the state (update available, up to date,
  check failed, not checked yet, or not comparable), the running and newest versions side by side,
  and the note on how the check runs.
- The watch records whether its last refresh got an answer, so a failed check is told apart from one
  that has not run.
- When a newer release exists, "How to upgrade" lists one copyable command per install method the
  upgrade docs describe: `tix update` for a release binary, `podman pull` of the new tag for the
  container, `go install ...@vX` for the Go toolchain, and a line for package managers.
- The notes render as Markdown through `internal/markdown`, in a readable column, without the
  release heading the section title already names. A `www.` address is stripped with the other links.
- One column at the form measure, spacing tokens throughout, a single-column phone layout.

## Impact

- `internal/web/release.go`, `internal/web/session.go`, `internal/web/templates/release.html`,
  `internal/web/assets/app.css`, `internal/i18n`.
- No route, service method, CLI flag or API changes. The footer link is unchanged.
