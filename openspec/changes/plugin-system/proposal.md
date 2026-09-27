# A plugin system that adds commands without adding an ABI

## Why

People want tix to do things tix should not do. A team wants `tix standup` to read yesterday's
transitions and post them somewhere; another wants `tix jira-pull` against a tracker tix does not sync
with; a third wants a custom field validated by a rule only their own policy knows. None of these belong
in a binary that ships six cross-compiled targets and freezes `internal/core` against the standard
library.

Out-of-process extension already exists, and this proposal does not re-propose it. A persisted outbox
gives gap-free `since_seq` resume, webhooks are HMAC-signed, API tokens are scoped, and every command
answers `-o json`. Anything that reacts to what happened is already served: a webhook receiver is the
right shape for it and needs nothing new from this change.

What none of that provides is a way for somebody else's code to be *reached from tix*. There is no
`tix <their-verb>`. A user who wrote the standup tool has to remember it is called
`~/bin/tix-standup-thing`, it appears in no `tix --help`, completion does not know it exists, and
nothing records what they installed or where it came from. The gap is discovery and invocation, not
transport.

## What Changes

- **A plugin is an executable tix invokes.** The model `git`, `kubectl` and `gh` use. It talks back to
  tix through the documented CLI and the HTTP API, both already stable contracts. No ABI, no cgo, no
  shared types, any language.
- **Installation is explicit.** `tix plugin install` takes a path, into a tix-owned directory. `$PATH`
  is never scanned. A plugin that is not in the manifest does not run.
- **A manifest** records every installed plugin: its name, its version, where it came from, the digest
  of the executable, the moment it was installed, and whether it was granted a token.
- **`tix plugin list`, `tix plugin info`, `tix plugin remove`** manage what is installed.
- **Dispatch on an unknown verb.** `tix foo`, matching no built-in command and matching an installed
  plugin, runs that plugin. A name colliding with a built-in never shadows it.
- **Connection context is handed over as environment**, resolved once by `internal/connect`, so a
  plugin never re-implements the five-layer configuration precedence.
- **No credential by default.** A plugin is given the target, not the token. A plugin that needs the
  API is granted a scoped token at install time, by an operator who is shown the scopes first.
- **The plugin's exit code becomes tix's exit code**, and the output format is passed through, so the
  codes documented in `docs/scripting.md` keep meaning what they say.

Explicitly out of scope: plugins do not register Cobra subcommands into the tree, do not appear in
`tix docs --table`, do not enter the capability registry, and do not contribute value completions.
Each of those is a decision with a cost, recorded in `design.md`.

## Impact

- New: `internal/plugin` (manifest, resolution, dispatch), `cmd/plugin.go`, `docs/plugins.md`.
- Changed: the root command's unknown-command path, and the row filter in `tix docs --table`.
- Unchanged: `internal/core`, `internal/service`, `internal/capability` and every storage package. A
  plugin is a caller, indistinguishable at the service layer from any other CLI or API caller.
