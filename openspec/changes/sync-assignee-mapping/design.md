# Design

## The users section

```yaml
users:
  map:
    Jane Doe: jane            # a handle
    j.smith: john@corp.test   # or an email
  match: [email, handle, name]
  email_field: _embedded.assignee.email
  unknown: unassigned          # or placeholder
```

Resolution order for one assignee value: the explicit `map`, then each `match` strategy in order, then the
`unknown` policy. `email` compares the value, and the value at `email_field` when the record carries one,
with the emails of the tenant's users, ignoring case. `handle` compares with actor handles, ignoring case.
`name` compares with actor and user display names, ignoring case, and only counts when exactly one actor
matches, so two people called Alex never get each other's work. `match: []` turns the automatic strategies
off. Omitting `match` means the default list. A map entry naming nobody is reported once and the value falls
through to the strategies.

## One directory per run

The first assignee a run meets loads the tenant's actors (paged by handle through `ListActors`) and the
user row of each user actor (`GetUser`, since a user and its actor share one id). The run keeps that
directory and a per-value answer cache, so a large import costs one directory read plus one lookup per
distinct value, not per record. Every read goes through the tenant-scoped transaction, so another tenant's
users can never match. `GetUserByEmail` is not used: it lives on the unscoped transaction.

## Placeholders

A placeholder is an actor of kind `user` with handle `ext-<slug of the value>`, the external value as its
display name, and nothing else: no user row, so it cannot sign in; no membership, so no role; no scopes. It
is the same shape as an actor whose account was deleted, which the product already handles. The run looks
the handle up before creating it and reuses an actor it finds there only if that actor has no account, so
a second run reuses the first run's placeholder, and a real person who happens to hold the handle is never
taken over (the value is then left unassigned with a warning).

Placeholders are left out of the automatic strategies, so a value keeps being reported as unmatched until a
real account matches it. When that person joins, the next run matches them by email or name and the update
moves the task from the placeholder to them.

## Reporting

`SyncResult.UnmatchedUsers` holds every value that reached the `unknown` policy, sorted and deduplicated,
in a dry run and a real one. One warning, added when the run finishes, counts and lists them; it says
"would" in a dry run and names placeholders under that policy. The per-record "has no tix actor" warning is
gone: it was capped with every other warning and drowned large imports.

## Parent links

HAL sources link a parent as `/api/v3/work_packages/42` while the identity field is the plain `42`. The
parent is looked up as given first, so a source whose identity is itself a link keeps matching. When that
finds nothing and the value is an absolute path or an http(s) url, its last path segment is tried. No new
mapping key is needed.

## The OpenProject default mapping

`internal/sync/openproject/default.yaml`, embedded and returned by `DefaultMapping()`. `project:` holds the
placeholder `CHANGE-ME`, which validates but names no project, so a run refuses it until it is edited.
Statuses map onto the built-in workflow (`todo`, `doing`, `blocked`, `done`, `cancelled`). `unmapped.preserve`
is false: a work package is mostly `_links` and `_embedded` HAL plumbing, and preserving it would put dozens
of link custom fields on every task. The useful links (project, author, accountable) are mapped to custom
fields by name instead. Unknown types are kept in the `op_type` custom field rather than becoming tags.
