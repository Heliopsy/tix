# Design

## Deriving authority without restating it

`internal/architecture`'s `TestAuthzIsCalledOnlyByService` holds AGENTS.md's invariant: only
`internal/service` may import `internal/authz`. So the terminal cannot ask the policy, and the question it
needs answered is not "may this actor perform this action" but "is there anything on this screen this actor
may do".

Three facts already exist in three places. `internal/authz` knows action to scope. `internal/service` names
the action at each call site. `internal/capability` knows which view an operation is reachable from. Only
the middle one was unexported knowledge, so the chain could not be walked.

The registry gets the missing link: `Operation.Scope`, the scope an actor must hold for the service to
permit that operation. It is not a hand-written opinion. `internal/capability/authority_test.go` calls every
method of `core.Service` reflectively, against the real service over a temporary SQLite database, as an
actor holding no scope at all, and reads the scope back off the `forbidden` error the policy produced. A
declared scope that nothing enforces fails, and an enforced scope nothing declared fails. The empty scope
means "needs a session and nothing more", and that too is proved rather than assumed, because the probe
requires the call not to be refused for want of a scope.

Two operations resist a single answer and say so instead of guessing. `ImportBundle` authorises each
component kind its bundle carries, so no one scope describes it; `scopeVariesByInput` names it with the
reason, and the guard requires such an operation to be bound to no view, since it could not be gated.

`capability.TUIAccess(actor)` then derives which views a reader may enter, and the caller passes the result
into `tui.Config`. `internal/tui` holds a `map[string]bool` and a rule for reading it, no scopes and no
actions. Because the answer is a value computed once from the actor the session already holds, gating a
keystroke costs no call and no query.

## Why the caller resolves it rather than the interface

`internal/capability` imports `internal/web` for its route constants, and `internal/web` reaches
`internal/service` and through it `internal/authz`. Importing the registry from `internal/tui` would drag
the browser, the API router and the policy into a package whose dependencies are `core`, `output` and
`query`. Both production callers already reach that far, so they resolve it and hand it over.

The trade is a field a caller can forget. The default is therefore closed: a nil `ViewAccess` offers only
the views that read nothing from the service. A caller that forgets loses navigation, which
`internal/integration/tui_matrix_test.go` sees at once; the other default would hand every reader every
screen. `TestNoAccessOffersNothingBeyondTheLocalViews` pins that direction.

## What "offered" means

A view is offered when the reader may perform at least one of the reads bound to it. Not all of them: a
viewer who may list tasks but not subscribe should still see the board, and the board says "disconnected"
rather than refusing to open. Not "any operation": a view whose writes are permitted but whose reads are not
has nothing to draw.

A read needing no scope counts. The tenant view is reachable that way and should be, because asking a
tenant who the actor is there is the only way to tell a tenant that does not exist from one this actor
cannot see. `TestEveryTUIViewHasAReadToOfferIt` keeps a view from binding only writes, which would hide it
from everyone permanently.

Three views are exempt because they read nothing a scope covers: the settings screen, the help overlay, and
the project list. The project list is where every session starts and where going back ends up, so there is
nowhere to send a reader refused it; the list's own empty state says what happened.
`TestLocalOnlyViewsReadNothingFromTheService` holds that list to exactly those three.

## One door

The gate is in `enterView`, which every entry already goes through, rather than beside each key. A view
added later is gated by existing rather than by its author remembering to ask.
`TestEveryGatedViewResolvesToARegistryName` closes the two silent failures this shape allows: a view whose
name the registry does not declare would be refused forever, and a registry view the interface cannot name
would be gated by nothing.

## A query-discriminated address

`internal/httpapi/handlers_project.go` serves `DELETE /api/v1/projects/{ref}` and branches on an `archive`
query flag, with deletion as the default branch. The registry declared both operations at the bare pattern,
which meant it could not say which one a reader reaches and named the destructive branch twice.

`Route` gains `Query`, and `Route.Address()` includes it. `project.archive` is declared at
`DELETE /api/v1/projects/{ref}?archive=true`, and `TestNoTwoOperationsShareOneAPIAddress` fails when two
operations collide at one address, so a future operation cannot shadow an existing one without the build
saying so.

## Limitations, which are not exemptions

An `Exemption` records a binding that is absent. Three bindings are present and reach less far than the same
operation reaches elsewhere: the browser's task screen walks one level of the subtask tree where the CLI and
the API walk any depth, and the browser's token and key lists are pinned to the signed-in actor where the
CLI and the API take another actor's identifier. Recording those as exemptions would collide with the
binding being declared, so `Limitation` carries them, and a guard requires each to name a surface the
operation actually binds.
