# Design

## Where each level lives

- Global: the deployment configuration, handed to the service as `WithTimeDefaults` from
  `output.time_format` and `output.timezone`.
- Tenant: two columns on `tenants`. Edited through `UpdateTenant`, which already carries the tenant's
  other settings and their authorization.
- User: two columns on the global `users` row, not on a tenant actor, so one person reads one format in
  every tenant. Read and written through a narrow `TimePrefsTx` rather than `userColumns`, so the
  feature does not widen every user query.

An automated actor has no users row. It reads the tenant's values and `SetTimePrefs` refuses it,
the way a second factor is refused to a caller who is not a person.

## Resolution

`core.ResolveTimePrefs(global, tenant, user)` picks each field from the strongest level that holds one
and names the level. It is pure and in `core`, so the service, the web and the tests share it.

## The web

`wrap` resolves the reader's settings once per request and stores the style in the request context;
template sets are cached per resolved style and language as before. `GET /settings` and
`/admin/tenant` show each level's control with an Inherit option that names what it would give.

The cookie migration runs in the same step. It writes only fields the account does not hold, so a
choice made on purpose elsewhere is newer than any cookie. The cookies are always expired, even when
nothing moved, so a person who is not a user (and so cannot hold a preference) is not left carrying
one that is never read.

No per-browser override is kept. The only reason to have one is a shared machine, and a person signs
in as themselves there anyway.

## Terminal and command line

The command line and the terminal interface ask the service once per run, and only for human output.
A value set on the machine (any layer above the shipped default) wins field by field, which is what
the terminal settings screen writes. The settings screen shows the account's value and its source for
a field the machine leaves unset.

## The tooltip

`when` and `whenRel` write `<time class="ts" datetime tabindex="0" title aria-label data-full>`. The
title covers hover, the label covers screen readers, and a CSS tip on `:focus` covers keyboards and
touch, where a title never shows. `tt` translates a sentence whose argument is such an element. A
timestamp inside a link or another attribute stays a plain string, because a focusable element may not
sit inside a link.
