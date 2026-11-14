# A token can require a claim before it changes a task

## Why

Several agents in one project shared a single admin token. Some moved tasks they had never claimed,
and the history could not tell the agents apart, because every change came from the same token and
the same actor. A rule that every move needs a claim would break the ordinary human flow, where a
person drags a card or raises a priority without claiming anything. The rule has to be opt-in, per
token, so people and every other token behave exactly as before.

## What Changes

- An API token carries `require_claim`, false by default. It is set when the token is minted:
  `tix token create NAME --require-claim`, `require_claim` on `POST /api/v1/tokens` (and so on the
  MCP tool generated from it), a checkbox on the browser's issue form and a field on the terminal
  interface's token form. Token listings, the issued token and `tix token ls` show it.
- Tokens have no update path, so the flag is fixed for a token's life. `tix token rotate` carries
  it over to the replacement, with the name, scopes, project and lifetime.
- A caller authenticated with a token whose `require_claim` is set may change a task only while its
  actor holds a live claim on it. The service enforces it, so the CLI, the HTTP API, MCP, SSH and
  the browser all get it. It gates:
  - a transition, a route and a bulk move (`tix task mv`, `tix task done`, the board);
  - a field edit, a checklist tick and adding or removing a tag;
  - a move to another project;
  - a delete, every task of a cascade included;
  - every task of a bulk action, each refused on its own as bulk already reports refusals.
- It does not gate claiming (`claim next`, `claim task`, with `--start`, which claims and moves in
  one step), creating a task, commenting, reactions, reading, or releasing its own claim.
- A refusal is permission denied (HTTP 403, exit code 5) with a message that says what to run:
  `this token requires a claim on ABC-12 before changing it; run: tix claim task ABC-12 --start`.
  A dry run of a move refuses the same way, because it runs the move's own checks.
- A move into a terminal state under the token's own lease token still releases the lease.

## Impact

- One migration adds `require_claim` to `api_tokens`, 0 for every existing token.
- `core.APIToken` and `core.CreateTokenInput` gain `RequireClaim`.
- No new `Service` method. `api/openapi.yaml` gains the field on the token schemas.
- Docs: the agent guide gains a short recommendation for running AI agents.
