# Tasks

## The registry

- [x] `capability/capability.go`: `Operation.Scope`, `Operation.Permits`, `Operation.Reads`
- [x] `capability/capability.go`: `Route.Query`, `Route.Address`, `Route.Path`
- [x] `capability/capability.go`: `Limitation`, `Shortfall`, `LimitationFor`, `Limitations`
- [x] `capability/capability.go`: `TUIViews` and `TUIAccess`
- [x] `capability/registry.go`: the scope every operation needs, one per entry
- [x] `capability/registry.go`: bind `actor.show` to the detail view and drop its terminal gap
- [x] `capability/registry.go`: attribute `event.subscribe` to the activity view
- [x] `capability/registry.go`: honest browser gaps on `identity.whoami` and `actor.show`
- [x] `capability/registry.go`: the archive branch's query-discriminated address
- [x] `capability/registry.go`: browser limitations on `task.tree`, `token.list` and `sshkey.list`
- [x] `web/routes.go`: the two matching browser exemptions

## The terminal interface

- [x] `tui/authority.go`: `ViewAccess`, the local-only views and `canReach`
- [x] `tui/model.go`: hold the set, and refuse an unreachable view in `enterView`
- [x] `tui/keys.go`: `GlobalHelp` filters by what is offered, and documents the statistics key
- [x] `tui/scheme.go`: `viewName` names the statistics view
- [x] `tui/view.go`: the overlay filters, and the duplicated doc line goes
- [x] `cmd/tui.go`, `sshd/session.go`: resolve the reachable views for the session's actor

## Guards

- [x] `capability/authority_test.go`: every declared scope proved against the real refusal
- [x] `capability/authority_test.go`: the probe is proved able to see a refusal at all
- [x] `capability/parity_test.go`: no two operations at one address
- [x] `capability/parity_test.go`: every view carries an operation, and every view carries a read
- [x] `capability/parity_test.go`: the gap counts per surface
- [x] `capability/parity_test.go`: every limitation names a bound surface
- [x] `tui/access_test.go`: every gated view resolves to a registry name
- [x] `tui/access_test.go`: the local-only carve-out is exactly three views
- [x] `tui/access_test.go`: no access supplied offers nothing beyond them
- [x] `tui/access_test.go`: the five authorities that connect, and what each is offered
- [x] `tui/access_test.go`: the overlay names only offered views
- [x] `integration/tui_matrix_test.go`: the real interface driven as a restricted reader
