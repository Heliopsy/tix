# Multi-hop transitions on every surface

## Why

"transition tui does not allow hops just single transition fix that i told you that cli tui and api/web
need to allow same features"

Multi-hop existed in exactly one control: the row menu on the browser's task list. It computed the
routes, showed the path and the hop count before anything was pressed, and submitted the whole route.
Everything else moved one state at a time.

| Control | Offered |
| --- | --- |
| web task list row menu | routes, with path and hop count |
| web task detail "Move to" | direct targets only |
| web board card move | one state |
| TUI `t` | direct targets only |
| CLI `tix task mv` | one state |
| HTTP API | one state |

The reason is structural. The route computation lived in `internal/web/flow.go`, which is why no other
surface had it, and the flash it wrote ("task moved doing → done, one step at a time") could only ever
describe moves five of six controls could not make.

Copying the walk into each surface would give four implementations of "which states can I reach and how".
This repository has already had that failure once, with ten false claims on its website traced to one
fact written out in several places.

There is a second report attached to the same change:

"i need to be sure each transition logs the correct amount of subtransitions of more than 1"
"add tests to the audit for that too"

A route through three states must write three audit entries and emit three events, in order, and nothing
else. That is the reason each hop stays an ordinary transition, and it is what makes the trail worth
reading: a task the trail says passed through a state really did pass through it, and one that did not
must not appear to have.

## What Changes

- **Reachability moves into `internal/core`.** `core.Routes`, `core.FindRoute`, `core.ParseRoute` and the
  `core.Route` type are pure graph logic over `core.WorkflowDefinition`, needing no service and no store.
  `internal/web/flow.go` keeps only presentation: labels, the CSS modifier, the path as a sentence.
- **Applying a route moves into `internal/service`.** `TransitionRoute` walks the route by calling
  `TransitionTask` once per hop, so each hop keeps its own transaction, audit entry and event. The loop
  used to sit in the web handler, which is business logic in a surface.
- **`task.route` is its own operation in the capability registry.** That is what makes the registry able
  to notice this defect: an operation it names must be bound or exempted in writing on all four surfaces,
  where a *capability of* an existing operation was invisible to it.
- **A route that stops part way is a result, not a failure.** It names where it stopped and why. Only a
  route refused on its very first hop is an error, because then nothing happened.
- **An ambiguous destination is refused.** When two routes of the same length reach a state, no route is
  chosen. The refusal names both and asks the caller to name one, written `doing>done`. Choosing would
  walk the task through a state nobody named, and that write is visible to webhooks and subscribers.
- **Every surface offers routes.** The TUI picker lists them after the adjacent states, so a single hop
  still costs one digit. `tix task mv REF STATE --hops` finds the route, prints it, then applies it, and
  honours `--dry-run` by printing and writing nothing. `POST /api/v1/tasks/{ref}/route` accepts a route or
  a destination. The browser's detail screen and board gain what its list already had.
- **Drag and drop on the board stays single-hop.** A drop names a column and nothing else, so lighting a
  far column up would walk a task through states the reader never saw.

## Impact

- Affected specs: `task-transitions`
- Affected code: `internal/core/reachability.go`, `internal/core/route.go`, `internal/core/service.go`,
  `internal/service/task.go`, `internal/client/tasks.go`, `internal/wire/wire.go`,
  `internal/httpapi/handlers_task.go`, `internal/web/flow.go`, `internal/web/taskdetail.go`,
  `internal/web/projects.go`, `internal/web/templates/task.html`, `internal/web/templates/board.html`,
  `internal/web/assets/decide.js`, `internal/web/assets/live.js`, `internal/tui/prompt.go`,
  `internal/tui/commands.go`, `cmd/task.go`, `cmd/route.go`, `internal/capability/registry.go`,
  `docs/tui.md`, `docs/commands.md`, `docs/api.md`, `docs/workflows.md`, `skills/tix/SKILL.md`
- `core.Service` gains one method, so every implementation of it grows one: `service.Local`,
  `client.Client` and the test stand-ins.
- Additive on the wire. `POST /api/v1/tasks/{ref}/transition` is unchanged, and a form or a client that
  submits one state keeps working.
