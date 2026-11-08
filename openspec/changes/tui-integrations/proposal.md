# Integrations in the terminal

## Why

The capability registry records twelve terminal gaps for the parts of a tenant that talk to the world
outside it: webhook endpoints and their delivery log, external sync sources, and the retention policy
with the prune that enforces it. A tenant administrator working over SSH had to leave the interface for
`tix webhook`, `tix sync` and `tix retention`, or for the browser, to do any of it.

## What Changes

- A new terminal view, `integrations`, opened with `J` from anywhere. It has three tabs, webhooks, sync
  and retention, and `←`/`→` move between them.
- A tab is offered only when the reader may perform its read: `webhook:admin` for webhooks,
  `sync:admin` for sync and `tenant:admin` for retention. The view itself is offered when any one of them
  is, through the same `capability.TUIAccess` every other view uses.
- Webhooks: the endpoints and the newest page of deliveries under one cursor. `n` registers an endpoint,
  `e` edits the selected one, `X` deletes it behind a confirmation, `enter` redelivers the selected
  delivery. A secret tix generates is shown once, on the tab, the way the browser shows it.
- Sync: the sources with what a run on the server would use and what it still lacks. `n` configures a
  source, `e` sets up the selected one, `X` deletes it behind a confirmation, `enter` runs it from a form
  holding dry run (on by default) and full. A form can check a source without saving it. The run's
  result, or the check's, is drawn on the tab.
- Credentials and signing secrets are write-only, as on the web: a new form field kind gathers them
  masked, they are never seeded into a form, and a blank one keeps what is stored.
- Retention: the three windows. `e` edits them, a blank window keeping its value. `enter` prunes from a
  form holding dry run (on by default) and a limit; a prune that writes asks first.
- The registry binds all twelve operations to the new view, so the parity test enforces them, and the
  recorded terminal gap count falls from 44 to 32.

## Impact

- `internal/tui`: new `integrations.go` and `integrations_view.go`, a masked field kind and a negated
  field condition in `form.go`, a `J` binding, and one line in each of the view, key and message
  switches.
- `internal/capability/registry.go`: twelve gaps become bindings; the gap count and the docs test's
  number words follow.
- docs/tui.md and docs/README.md: the view, its keys, the view count and the gap section.
