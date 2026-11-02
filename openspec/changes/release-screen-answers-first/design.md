# Design

## One answer, five states

`upgrade.Status()` derives the state from what the watch already caches: `newer` when the latest tag
is ahead, `current` when it is not and both versions parse (or are equal), `unknown` when a version
cannot be ordered, `failed` when the last refresh got no usable answer, `pending` when none has
finished. A version this build cannot compare is not called current, for the same reason
`newerRelease` stays quiet on a string nobody parsed.

The watch still fails silent: nothing is logged and the page does not warn on its own. The failed
state only changes the card's wording, and it uses the quiet surface, not a warning colour, because
an isolated network is the ordinary case for a self-hosted tracker.

## Install methods

The page cannot reliably tell how the server was installed: `selfupdate.Detect` tells a Go install
from a release archive, but a container's binary looks like a release archive. So every method is
listed and the reader picks theirs. The commands come from `docs/upgrading.md` and the README and
carry the version on offer. They are shown only when a newer release exists.

## Notes as Markdown

The notes go through the same renderer as a task description, which passes no raw HTML and loads no
image. Links were already removed when the notes were cached; `www.` addresses are now removed too,
because the renderer's linkify would otherwise turn them back into links. The leading
release-please heading (`## 0.30.0 (date)`) is dropped when it names the release, since the section
heading says the same thing.

## Older versions

The feed answers with the newest release only, so there is one set of notes and nothing to collapse.
