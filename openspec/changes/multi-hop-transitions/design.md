# Design

## Where the graph lives

`internal/core`. It is a pure function of `core.WorkflowDefinition`, needs no service and no store, and
`internal/core` imports only the standard library, which that satisfies. Every surface asks the same
function, which is the point: four copies of "which states can I reach and how" would drift, and this
repository has already paid for one fact written out in several places.

`core.Routes(def, from)` walks breadth first. A neighbour is therefore always offered as one hop and
never as a detour, and a state is only ever reached at its own shortest distance, which is what stops a
workflow's cycles (todo, doing, todo) walking forever.

Two caps sit on the walk. `MaxRouteSteps` (16) bounds depth, so a crafted field cannot ask for an
unbounded run of writes. `maxRoutesPerState` (4) bounds how many equally short routes to one state are
enumerated: a workflow is small, but the number of equal-length paths through a graph is not bounded by
its size, and a menu with forty ways to reach one state is not a menu.

`Routes` returns one entry per distinct shortest route, not one per reachable state. A menu therefore
offers the journey rather than the destination, and a menu can never be ambiguous, because pressing an
entry names the whole route.

## What stays in the web

Labels, the `is-multi` CSS modifier, the hop count as the button spells it, and `Path()`, which prefixes
the state the task is in now so a reader consents to the journey rather than discovering it in the trail.
`flowRoute` is a thin wrapper over `core.Route` holding exactly those.

## Where the walk happens

`internal/service`. `TransitionRoute` calls `TransitionTask` once per hop.

That is a deliberate move of the loop out of `internal/web`, where it was business logic in a surface.
It also gives the per-hop guarantee a single place to be stated and a single place to be tested: a
surface that wires a route up wrongly now fails against a rule the service itself holds, rather than each
surface being spot-checked for the same thing.

`TransitionRoute` authorizes once under the transition action and then reads the task and its workflow in
one read transaction, rather than calling `GetTask`. Calling `GetTask` would make the operation need
`task:read` as well, and the capability registry records one scope per operation, which is asserted
against what the service actually enforces.

## Decisions

**Each hop is an ordinary transition.** A route through three states writes three audit entries and emits
three events, exactly as making those moves by hand would. Collapsing them into one write would be faster
and would be a lie: a task that the trail says passed through a state really did pass through it.

**An ambiguous destination is refused.** Not resolved, not shortest-wins. Silently routing a task through
a state nobody chose writes audit entries and fires events for moves nobody asked for, and those reach
webhooks and `tix watch` subscribers. The refusal names both routes and the syntax for naming one, so it
is actionable rather than merely correct.

Ambiguity is only possible when a caller names a *destination*. A menu submits a whole route and never
meets it. So the refusal lives in `core.FindRoute`, which the CLI, the API's `to` form and any other
destination-only caller reach, and menus are unaffected.

**A partial route reports where it stopped.** The hops that landed are real and the task is somewhere.
Reporting only the failure would leave a reader with a task in a state nobody chose. `core.RouteResult`
carries `Applied`, `Stopped` and `Reason`, and `Sentence()` is what a surface with one line to spend
prints. Only a route refused on its first hop is an error, because then nothing happened.

**`task.route` is a registry operation, not a flag on `task.transition`.** This is the answer to "can the
parity test be strengthened". The registry asserts that a binding exists, which is why it was satisfied
while one surface offered routes and three did not: multi-hop was a capability *of* an operation, and the
registry only knows operations. Making it an operation puts it inside the existing check, at no new
machinery: every surface must now bind it or record an exemption in prose, and `TestEveryLimitationIsJustified`
keeps any recorded shortfall attributable. `internal/web`'s own parity test, which walks `core.Service`
by reflection, gained it for free.

## Surface shapes

**TUI.** The picker is a numbered list, and a route may not make a single hop cost more keystrokes than
it did. Because the walk is breadth first, the adjacent states come first, so `t` then `1` is still one
digit for the move a reader makes most often. A route sits further down the same list and names itself:
`Done via Doing (2 steps)`. That is the hop count shown before it applies.

**CLI.** `tix task mv REF STATE --hops`. The route is resolved with `core.FindRoute` in `cmd/`, printed
to stderr, and then submitted as an explicit route. Resolving in the command rather than letting the
service resolve `To` is what makes `--dry-run` honest: the route a dry run prints is the route a real run
takes, because both come from the same call. Naming the hops by hand is still accepted — `doing>done` as
the STATE argument — which is what the ambiguity refusal asks for.

`-o json` reports `from`, `route`, `applied`, `hops` and `stopped` per reference, so a script can see the
journey taken rather than infer it.

**HTTP API.** `POST /api/v1/tasks/{ref}/route` takes `{"route": [...]}` or `{"to": "..."}`. The existing
`POST .../transition` is untouched. A partial route is 200 with the result saying where it stopped, not
an error envelope, for the same reason the browser flash says it: the hops that landed are real.

**Web.** The detail screen's "Move to" and the board card's Move select now carry routes, valued as the
route (`doing>done`) and labelled with the path. Drag and drop stays single-hop: a drop names a column
and nothing else, so `tix.legalStates` ignores any option value containing the separator.
