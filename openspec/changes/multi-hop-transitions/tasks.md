# Tasks

## 1. Reachability in core

- [x] 1.1 `internal/core/reachability.go`: `Route`, `Routes`, `FindRoute`, `ParseRoute`, `StateLabel`,
      `RouteSep` and `MaxRouteSteps`, breadth first, with both caps.
- [x] 1.2 `internal/core/route.go`: `RouteInput` with its validation, `RouteResult` with `Partial`,
      `Hops`, `Status` and `Sentence`.
- [x] 1.3 `internal/core/reachability_test.go`: the walk, the cycle, the undeclared edge, the fork that
      is refused, the refusal naming both routes, the caps.

## 2. Applying a route in the service

- [x] 2.1 `internal/core/service.go`: `TransitionRoute` on `TaskService`.
- [x] 2.2 `internal/service/task.go`: `TransitionRoute`, authorizing once under the transition action,
      reading the task and its workflow in one read, then one `TransitionTask` per hop.
- [x] 2.3 `internal/service/route_test.go`: one audit entry and one event per hop, exact count and exact
      order, for one, two and three hops and for a named destination.
- [x] 2.4 `internal/service/route_test.go`: the partial route writes only the hops that happened; the
      route refused on its first hop writes nothing.
- [x] 2.5 `internal/service/route_test.go`: the ambiguous destination is refused, names both routes,
      writes nothing, and the named route then works.

## 3. HTTP API

- [x] 3.1 `internal/wire/wire.go`: `RouteTaskRoute`.
- [x] 3.2 `internal/httpapi/handlers_task.go`: `handleTransitionRoute`, 200 with the result for a partial
      route, the existing error envelope otherwise.
- [x] 3.3 `internal/client/tasks.go`: `TransitionRoute`.
- [x] 3.4 `internal/httpapi/handlers_route_test.go`: a route over the wire writes one entry per hop; the
      partial route; the ambiguous refusal; the single-state endpoint unchanged.

## 4. Web

- [x] 4.1 `internal/web/flow.go`: `flowRoute` wraps `core.Route`; the walk and the parser are gone.
- [x] 4.2 `internal/web/taskdetail.go`: the handler calls `TransitionRoute` and flashes the result's own
      sentence; the detail screen carries routes.
- [x] 4.3 `internal/web/projects.go`: the board card carries routes; `moveCard` submits a route;
      `targetsFrom` and the unused `moves` template function are removed.
- [x] 4.4 `internal/web/templates/task.html`, `board.html`: the selects carry routes.
- [x] 4.5 `internal/web/assets/decide.js`, `live.js`: drag and drop reads only the one-hop options.
- [x] 4.6 `internal/web/jstest/live.test.mjs`, `projectcrud_test.go`, `flow_internal_test.go`: follow.

## 5. TUI

- [x] 5.1 `internal/tui/prompt.go`: `TransitionChoices` offers routes, adjacent states first, each route
      naming its path and hop count.
- [x] 5.2 `internal/tui/commands.go`: `transition` submits a route and reports what came back.
- [x] 5.3 `internal/tui/route_test.go`: the picker against a real service; a route writes one entry per
      hop; a single hop is still `t` then `1`.

## 6. CLI

- [x] 6.1 `cmd/task.go`: `--hops` on `task mv`.
- [x] 6.2 `cmd/route.go`: resolve the route, print it, apply it; `--dry-run` prints and writes nothing;
      `-o json` reports the route and the hops.
- [x] 6.3 `cmd/route_test.go`: the printed route, the dry run writing nothing, the JSON shape, the
      ambiguous refusal, one audit entry per hop.

## 7. Registry and docs

- [x] 7.1 `internal/capability/registry.go`: `task.route` with all four bindings; `task.transition`'s web
      binding moves to the complete route, which is the single transition the browser still makes.
- [x] 7.2 `internal/capability/authority_test.go`: sample arguments for the new method.
- [x] 7.3 `docs/tui.md`, `docs/commands.md`, `docs/api.md`, `docs/workflows.md`, `skills/tix/SKILL.md`.
