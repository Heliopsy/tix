# Design

## Resolving the token

`credential()` now calls `resolve()` itself. Resolution is cached per invocation, so this costs nothing and
removes the ordering dependency. The precedence is unchanged: `--token` and `TIX_TOKEN` are the flag layer of
`server.token`, then `TIX_SERVER_TOKEN`, `.env`, the current context and the configuration file.

## When a first run refuses

The guard sits in `dialWith`, the one place every command opens its target, and in `tix demo seed`, which dials
on its own. It fires only when all of these hold:

- the target is local and its origin is `default` (no flag, no context, no `database.dsn` from any layer);
- `target` is not `local`;
- no file exists at the default path.

So an existing database at the default path is an opt-in by itself. That keeps every install from before this
change working without a step, which matters more than catching the user who already has a stray database:
for that case the notice line and `tix whoami` are the safeguard.

The whole command is refused, reads included, because opening the target is what creates the database. A
terminal gets a question instead; shell completion never asks. Choosing "server" writes nothing and prints the
`tix ctx add` line. We do not prompt for a URL and token in place: the `/connect` page already prints the
exact line, and a half-written context is worse than none.

Exit 6 (`precondition_failed`) because the command is valid and the environment is not ready.

## The notice

Printed on standard error after the connection opens, whenever the target is the implicit default database,
including after opting in with `target: local`. It is not a diagnostic and `--quiet` does not hide it: agents
run with `-q`, and they are the reader who most needs it. `cli.target_notice: false` silences it. A target named
explicitly (`--db`, `TIX_DATABASE_DSN`, a context) prints nothing, so scripts that already say where they
write are unaffected.

## whoami

`tix whoami` was missing; `identity.whoami` was bound to `tix doctor`, which answers a different question.
It reports the target, the context, the tenant key (looked up from the tenant id, falling back to the
configured key), the user, and the token source as a name (`--token`, `TIX_TOKEN`, `TIX_SERVER_TOKEN`, the
`.env` path, `context NAME`, or `server.token in FILE`), never the value.

## tix login

`tix login` returns a browser session, which the API takes only as a cookie. It is not a credential for the
command line, and this change does not make it one. The docs and the skill now say to use an API token in a
context instead.
