# Design

## Subprocess, not linkage

A plugin is a file with the execute bit set. tix runs it with `exec`, hands it argv after the plugin's
own name, wires it to the caller's stdin, stdout and stderr, and waits. Everything it wants from tix it
gets by running `tix` again or by calling the HTTP API. That is the whole interface.

The value of this shape is that both halves of it are already contracts the project maintains. `-o json`
is on every command, the exit codes are documented, and the HTTP API is versioned. A plugin author needs
no header, no generated stub and no build of tix. A plugin can be a shell script.

The cost is a process per invocation and no way to extend anything that is not a command. A plugin
cannot add a column to the board, a field type, or a filter predicate, because those are evaluated
inside a running tix and a subprocess is not. That is the boundary, and it is drawn deliberately rather
than papered over: see the WASM note below for the part of it that has a future.

## Rejected: Go's `plugin` package

`CGO_ENABLED=0` is an architecture invariant in AGENTS.md, and the six-target release matrix depends on
it. `plugin` requires cgo, is Linux and macOS only, and refuses to load anything that was not built with
an identical toolchain and identical versions of every shared dependency. Adopting it would mean
abandoning the cross-compile matrix, dropping Windows entirely, and telling plugin authors to rebuild on
every tix release and every transitive dependency bump. It is rejected on the invariant alone; the
version-lock is the reason the invariant is worth keeping.

## Rejected for now: WASM via wazero

wazero is a pure-Go WebAssembly runtime: no cgo, sandboxed by construction, memory and fuel bounded.
It is the right answer to the thing subprocesses cannot do, which is in-process pure functions. A
custom field validator, a filter predicate, a board sort key: each is called thousands of times inside
one request, each should not be able to open a socket, and each is a pure function of its input. A
subprocess is the wrong shape for all three, and wazero is the right one.

It is not in this change because there is no such extension point yet. Field validation, filtering and
sorting are all internal today with no declared seam, and inventing a host ABI for hooks nobody has
asked for would be guessing. A plugin whose manifest declares a `wasm` kind is a compatible later
addition to the same manifest, and the manifest format is specified so that it can be.

## Rejected: hashicorp/go-plugin

It solves the real problem well: gRPC over a local socket, versioned protocol, in-process ergonomics
without in-process risk. It also brings gRPC, protobuf, go-hclog and their transitive trees into a
project that refused a router and refused an ORM and depends on almost nothing. The dependency weight
is not proportionate to adding `tix standup`. Rejected on cost, not on design.

## Security: what an unknown executable is trusted with

This is the part that matters, because getting it wrong is code execution as the user of a tool that
stores API tokens.

**No PATH scanning.** `git` and `kubectl` dispatch to any `git-*` or `kubectl-*` on `$PATH`. That makes
every writable directory on `$PATH` a code-execution primitive: drop `tix-status` next to a stale
`~/bin` entry and it runs the first time somebody typos. `gh` does not do this, and neither does tix.
A plugin runs only if it is named in the manifest and lives in the plugin directory.

**The manifest is the authority.** `$XDG_DATA_HOME/tix/plugins/manifest.json`, one entry per plugin,
written only by `tix plugin install` and `tix plugin remove`. It carries the name, the executable's
path relative to the plugin directory, its SHA-256 digest, the source it was installed from, the
install time, and the granted scopes if any. Resolution reads the manifest, never the directory
listing: a file dropped into the plugin directory by hand is not a plugin.

**The digest is checked before every exec.** A mismatch refuses the run and says so, naming the
plugin. This does not stop a determined attacker who can also rewrite the manifest, and it is not
claimed to. It catches the case it can catch: something replaced the binary after the operator
approved it.

**Permissions are checked before every exec.** The plugin directory and each executable in it must not
be writable by group or other, and must be owned by the user running tix. A directory anyone can write
into is the PATH problem with extra steps.

**A plugin is trusted with**: the connection target, the tenant, the output format the caller asked
for, and the caller's stdin, stdout and stderr. It is trusted to run as the user, with the user's
filesystem and network access, because it is an ordinary process and no sandbox is claimed. The
installation step is where the trust decision is made, and `tix plugin install` says so in as many
words before it writes anything.

**A plugin is not trusted with**: the caller's credential. `TIX_TOKEN` and `TIX_SERVER_TOKEN` are
removed from the environment handed to a plugin, always, even when the operator set them for their own
shell. This is the difference between "the plugin can do what I can do if I let it" and "every plugin
silently inherits my admin token".

**A plugin that needs the API gets its own token.** `tix plugin install --grant task:read,task:write`
mints a scoped token bound to that plugin, records the scopes in the manifest, and passes the value in
`TIX_PLUGIN_TOKEN`. The operator sees the scopes before the grant. `tix plugin remove` revokes it.
`tix plugin list` shows what each plugin holds, so the answer to "what did I give away" is one command.
Default is no grant, and the default is what a plugin installed without thinking gets.

## `tix docs --table` and the command table guard

`writeCommandTable` walks `root.Commands()` and emits a row for every available command whose
`GroupID` matches a registered group. `TestReadmeCommandTableMatchesTheCommandTree` regenerates that
block and holds `docs/commands.md` against it.

Plugin commands are **excluded** from that generation, and the exclusion is deliberate rather than a
side effect of leaving `GroupID` empty. If plugin rows were generated, the table would depend on what
happens to be installed on the machine that ran the generator: a contributor with a plugin installed
would commit rows nobody else can reproduce, and CI, with nothing installed, would fail the guard. The
committed table describes the binary, and the binary is the same everywhere.

Two things follow. Rows are filtered by an explicit marker rather than by absence of a group, so the
intent is legible at the filter. And because plugins never become Cobra commands in the first place
(dispatch happens on the unknown-command path), the filter is a guard against a future refactor rather
than load-bearing today; a test asserts the table is unchanged with a plugin installed.

`tix plugin` itself is a built-in command and does appear in the table, like any other.

## The capability registry does not cover plugins

`internal/capability` declares every `core.Service` method once and fails the build unless each carries
CLI, HTTP and Web bindings. A plugin command is not a `Service` method. It calls the service the same
way any external client does, through bindings that are already registered, so there is nothing new to
bind and no parity to prove.

This is stated here rather than left to be discovered by whoever first runs the parity suite after
adding a plugin and wonders why nothing failed. The registry is the map of what tix does. A plugin is
somebody else's program, and putting it on that map would make the map a lie about the product.

The one exception is `tix plugin` and its subcommands, which *are* built-in operations. If any of them
becomes a `core.Service` method it gets a registry entry with all three bindings, like everything else.
As specified they are local process management with no service call, so they do not.

## Completion: plugins are completed, arguments are not

tix completes more than flag names. `completeTaskRefs`, `completeProjectKeys`, `completeStatuses` and
`completeContexts` hit the resolved target and offer live values, including the states the project's
own workflow defines.

A plugin participates in exactly one place: `tix <TAB>` offers installed plugin names alongside
built-in commands, read from the manifest, with the plugin's one-line summary as its description.
Past that, **a plugin's own arguments are not completed at all**.

The cost is real and worth naming. A plugin taking a task reference gets no reference completion,
so `tix standup <TAB>` offers nothing where `tix task show <TAB>` offers live references. A plugin
author's options are to accept that, or to document their arguments and let users type them.

The alternative was a completion protocol: tix execs the plugin with a reserved argument on every
`<TAB>`. That was rejected because it means pressing TAB runs somebody else's program, synchronously,
inside the shell's completion path, on a keystroke rather than on a command. Completion is the one
context where a slow or hanging plugin is indistinguishable from a broken shell, and it is also the
context where the user has not decided to run anything yet. Name completion from the manifest costs
one file read and runs nobody's code.

## Connection context

`internal/connect` resolves one target from flags, environment, `.env`, the config file and the
defaults, and the result is either a local database or a server URL. A plugin must not redo that:
five layers reimplemented in a shell script will disagree with tix on the first edge case.

tix resolves the target once and exports it:

- `TIX_PLUGIN_API=1`, so a plugin knows it was run by tix and not by hand.
- `TIX_PLUGIN_NAME`, the name it was invoked as.
- `TIX_PLUGIN_VERSION`, the tix version running it, for a plugin that wants to refuse an old one.
- `TIX_PLUGIN_MODE`, `local` or `remote`.
- `TIX_PLUGIN_TENANT`, the resolved tenant.
- `TIX_PLUGIN_FORMAT`, the output format the caller asked for.
- `TIX_DATABASE_DSN` when the mode is local, `TIX_SERVER` when it is remote.
- `TIX_PLUGIN_TOKEN` only when a grant exists.
- `TIX_BIN`, the absolute path of the running tix binary, so `tix` in a script means *this* tix and
  not whatever is first on `$PATH`.

A plugin that re-invokes `"$TIX_BIN" task ls -o json` therefore hits the same target its caller did,
with no flags to forward and no precedence to reimplement.

The local case hands over a database DSN, which is the strongest thing in the environment: direct
store access, no scope enforcement, because the local path has no token layer. This is why the local
case is stated in the spec as its own requirement and why `tix plugin install` says it out loud. An
operator who does not want that runs plugins against a server target.

## Exit codes and output

The documented codes (0 success, 1 error, 2 usage, 3 not found, 4 conflict, 5 permission denied,
6 precondition, 7 external read) only mean something if they are not overwritten. tix propagates the
plugin's exit status unchanged, exactly as `tix claim exec` propagates its child's, and reserves no
code for itself in the plugin's range.

The failures that are tix's own happen before the plugin starts: an unknown name, a digest mismatch, an
unsafe permission mode, a plugin that is not executable. Those exit 1 or 2 and are distinguishable
because the plugin produced no output.

`-o json` is passed through as `TIX_PLUGIN_FORMAT` and the plugin is required to honour it: a plugin
run with `-o json` that writes a table is a plugin that breaks the script calling it. tix cannot
enforce this and does not try; it is stated as an obligation in the spec, and `tix plugin install`
is the point at which a person decides whether they believe it.

## Name collisions

A built-in always wins. Dispatch happens only on Cobra's unknown-command path, so a plugin named
`task` is never reached: `tix task` is the built-in, for ever.

Refusing the install outright would be tempting and is wrong, because the set of built-ins grows. A
plugin installed happily today would become unreachable when tix adds a command with its name, and a
refusal at install time cannot prevent that. So the rule is the same in both directions: install warns
if the name is already a built-in, `tix plugin list` marks a shadowed plugin as unreachable, and
`tix plugin info` says which built-in is in the way. The plugin stays installed and stays runnable
under a different name the operator chooses with `--as`.
