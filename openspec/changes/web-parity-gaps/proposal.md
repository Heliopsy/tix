# Web parity gaps

## Why

The capability registry records two browser gaps and three browser shortfalls. No screen says who the
reader is (`identity.whoami`), nothing opens an actor (`actor.show`), the task screen draws direct subtasks
only (`task.tree`), and the token and SSH key screens cannot show another actor's credentials although
the command line and the API can on the same scope (`token.list`, `sshkey.list`). Each one is a place
where somebody has to leave the browser to answer a question it should answer.

## What Changes

- Settings gains a Profile section, first, stating the reader's handle, display name, actor id, kind,
  role, tenant, scopes, whether they are a global admin and what this request authenticated with. It
  links to the reader's own actor screen. Email and memberships follow once whoami answers them.
- `/actors/{id}` is an actor screen: the actor's handle, name, kind and id, links to their tasks and, for
  a credential administrator, to their tokens and keys, and their recent activity for a reader holding
  `audit:read`. The directory's handles, a task's assignee and creator, a comment's author and a token's
  owner link to it.
- The task screen draws the whole subtask tree. Levels past the third load on request, a node with
  children collapses and expands, and without scripting the loaded tree is drawn open with a link to
  the subtask that holds the rest.
- The token and SSH key screens take an actor filter (`?actor=`, a handle or an id) that lists that
  actor's credentials, which `token:admin` already permits on every other surface.
- The two gaps and three shortfalls are removed from the registry, so the parity tests hold the
  bindings.

## Capabilities

### Modified Capabilities

- `web-ui`: the Profile section, the actor screen, the subtask tree and the credential actor filter.
- `capability`: the browser carries no gaps and no shortfalls.

## Impact

`internal/capability/registry.go` and its parity test, `internal/web` (`session.go`, `actors.go`,
`taskdetail.go`, `admin.go`, `routes.go`, `help.go`, templates, a new `assets/tasktree.js`, new CSS parts),
`internal/i18n`, `docs/web-ui.md`, `docs/tui.md` is unaffected.
