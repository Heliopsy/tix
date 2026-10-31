# Design

## What a ref is made of

A task reference is not stored. `tasks` holds `project_id` and `seq`, and every read joins
`projects` and renders `key-seq`. Changing `projects.key` therefore renames every reference at once:
no task, comment, dependency or event row needs rewriting. The places that hold a key rather than an
id were checked:

| Place | Holds | After a rename |
|---|---|---|
| `tasks`, `task_deps`, `comments`, `artifacts`, `task_stars` | ids | unchanged, refs render with the new key |
| `api_tokens.project_id` | id | a project-scoped token keeps its scope |
| `webhook_endpoints` | event types only | unchanged |
| `events` and `audit_entries` payloads | the key at the time | history stays as it was; the rename event names both keys |
| task bodies and comments | free text | not rewritten; text is not linked, pasted URLs redirect |
| CLI config `project:` default, sync mapping `project:` | a key | resolves through the alias |
| `tix_columns` and other web cookies | page names, not projects | unchanged |
| browser `localStorage` (sidebar, lanes) | widths and a boolean | unchanged |
| snapshot (`tix export`/`import`) | the key | an import naming an old key updates the renamed project |
| bundles | project template keys | a template naming an old key resolves to the renamed project |

## Aliases

A new table `project_key_aliases (tenant_id, key, project_id, created_at)` keyed by
`(tenant_id, key)` with `project_id` cascading on delete. Renaming `infra` to `ops` inserts
`infra → project`. Renaming back to `infra` deletes that alias row, because the key is the project's
primary key again.

Aliases are permanent for the life of the project. A key leaks into commit messages, chat and
bookmarks exactly as a task number does, and the task number counter already refuses to reissue a
number for that reason. Reissuing an old key to a different project would make yesterday's link open
somebody else's task. Deleting the project removes its aliases, and the keys become free again,
because then nothing remains for an old link to reach.

## Where the alias is followed

In the store's `GetProject` and `GetTask`. Both already resolve a human key; when the key misses,
they look the alias up and retry by project id. Following it there means every caller that turns a
user's key into a row (29 `GetTask` call sites, project lookups for fields, tokens, sync, stats,
claims, bundles, filters) follows it with no per-call change, and none of them can forget. Deciding
what may be renamed, and refusing reuse, stays in the service.

A side effect is wanted: the service's existing uniqueness check on create (`GetProject(key)` must
miss) now also refuses a key that is an alias of another project.

## Web redirects

The board, fields, task screen and task live fragment compare the key in the path to the canonical
key of what they loaded. When they differ and the request is a GET, they answer `301 Moved
Permanently` with the path rewritten and the query kept. A POST to an old path is served through the
alias without a redirect, so a form left open in another tab still saves.

## Confirmation

The settings form posts to `/projects/{key}/update` as before. If the key field differs from the
current key and the post does not carry `confirm_key`, the handler renders a confirmation page
instead of writing: it lists two example references before and after, explains that old links
redirect, and repeats every submitted field as a hidden input with `confirm_key=1`. Nothing needs
scripting.
