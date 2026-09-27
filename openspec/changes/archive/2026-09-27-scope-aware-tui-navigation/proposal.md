# Navigation the reader's scopes decide, and a registry that tells the truth

## Why

`tix ssh` is a client. People reach a board over SSH on the public demo, on their own server or on
somebody else's, and the terminal has to offer the whole product bounded by that person's permissions.
`internal/sshd/enrolled.go` already says so: it clears the key's scopes so a session's authority is its
membership and nothing else, and `visitorScopes` applies only to the demo path, which hands every new key
its own throwaway tenant.

The terminal interface read none of that. `grep -rn 'HasScope\|Scopes' internal/tui/*.go` returned nothing
outside tests. The model held the `Actor` and ignored its authority. That was invisible only because no
view yet needed a scope the sandbox grants, and it stops being invisible the first time an administration
view lands. The browser already states the principle: an entry leading to a refusal is worse than no entry.

The capability registry, which is where the terminal would have to learn what a view needs, was wrong in
both directions. It recorded a terminal gap for `actor.show` that the detail view has resolved all along;
it bound nothing to the activity view, which is fully implemented; it claimed the browser had no gaps while
two operations reach no browser reader at all; and it asserted `project.archive` and `project.delete` at one
address, where the branch a caller gets without the query flag is the destructive one.

## What Changes

- **The terminal offers a reader the views their scopes permit.** A view is offered when the reader may
  perform at least one of the reads the registry binds to it. `enterView` is the only door, so a view
  added later is gated by existing.
- **The help overlay agrees with what is offered.** It used to render every cross-view binding regardless
  of who was reading, and it never documented the statistics key at all.
- **Authority is not restated in `internal/tui`.** The registry records the scope each operation needs, a
  guard proves every one of those against the refusal the real service produces, and the caller resolves
  the reader's reachable views once per session. `internal/tui` may not import `internal/authz`, and a
  table of its own would have been a second opinion about permission.
- **`capability.Route` can express a query-discriminated address**, so two operations sharing one pattern
  hold two distinct addresses and a new operation cannot shadow an existing one silently.
- **The registry states where each surface actually falls short**: the terminal gap on `actor.show` is
  gone, the activity view is attributed, the browser carries two honest gaps, and three bindings that exist
  but reach less far than the CLI and the API carry a recorded limitation.

## Impact

- `internal/capability`: `Operation.Scope`, `Route.Query`, `Limitation`, `TUIViews`, `TUIAccess`, and the
  corrections above. The package stops being test-only and becomes what the terminal derives from.
- `internal/tui`: `authority.go` and the gate in `enterView`; the help overlay takes what is offered.
- `internal/web/routes.go`: two exemptions, so the browser's own list agrees with the registry's.
- `cmd/tui.go` and `internal/sshd/session.go`: resolve the reachable views for the session's actor.
- No change to `internal/authz`, `internal/service` or `core.Service`. The policy is unchanged; what is new
  is that a surface can now ask what it already enforced.
