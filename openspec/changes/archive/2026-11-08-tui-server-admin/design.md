# Design

## Knowing a reader is a global admin

`TUIAccess` and the model's action gate both ask `Operation.Permits`, which asks `Actor.HasScope`.
`HasScope(global:admin)` is true only for a token that names the scope, so a person signed in through a
session or an SSH key, who is a global admin, was never offered anything gated on it.

Authorization for global admin is not a scope check. `Policy.CanGlobal` asks the store whether the
actor's account is a global admin at the moment of the call, and requires a token to name `global:admin`
on top. `WhoAmI` runs exactly that check and reports it as `Actor.GlobalAdmin`, "a report for interfaces
deciding what to offer". The web sidebar already offers the Global admin area from it.

So `Permits` answers `a.GlobalAdmin` for an operation whose scope is `global:admin`, and the two callers
that build a terminal session (`cmd/tui.go`, `internal/sshd`) pass an actor whose `GlobalAdmin` was set
by `WhoAmI`, through `tui.WithStanding`. A failed `WhoAmI` leaves the actor as it was, which offers
nothing new. The service still decides every call; the report only decides what is offered. The policy
reads the store per call, so a revoke takes effect on the next call even though the offer was resolved
at the start of the session.

Considered and rejected: computing standing inside the model at `Init`. It would put a permission
decision in `internal/tui` that the callers' `Access` does not know about, and the two would disagree.

## Why the tenant pin does not prevent it

Every global admin service method names the tenant it acts on (`ListTenantUsers(ctx, tenant, ...)`,
`CreateTenantUser(ctx, tenant, ...)`, `AddTenantMember(ctx, tenant, ...)`, `DeleteTenant(ctx, ref)`)
and resolves it inside the service (`inTenant`, `reachTenant`). The web handlers call them from a session
pinned to one tenant, passing a `target` form field. The terminal does the same with the key under its
cursor. No connection is dialled to the other tenant.

The one real consequence of the pin is deleting the tenant the session is on: it would end the session's
own access mid-action. The web Global admin screen marks that row and offers no delete; the terminal does
the same and records it as a limitation on `tenant.delete`, since `tix tenant rm` can.

## Two views, not five

The web spreads the area over the Global admin screen, a tenant's users screen and the account log
screen. The terminal has one `globaladmin` view with three modes (tenants and admins, one tenant's users,
an account log) and Back walks out of a mode before it leaves the view. One view keeps the registry, the
view registry and the key tables to one entry each, which matters while several views are being added in
parallel.

The server view is separate because its audience is different: any tenant admin, not only global admins.

## Keys

The cross-view keys are `%` (server) and `!` (global admin). Every letter worth having is taken, and
symbols are least likely to collide with the views being added alongside. Inside both views the board's
own keys are reused, as the tenant screen does: `n` adds, `e` edits, `X` removes, enter opens, `/` asks
for an email whose account log to read. Each is gated on the operation it calls and listed once for the
footer, the help overlay and the keystroke.

## Secrets in a form

A new user's password is typed into a form. It is a text field with the form's shared `Secret` flag,
the one the people screen's password uses: its input is echoed as bullets and its summary on an
unselected row is masked, so a password never reaches the frame. The input's echo mode is reset when the
form closes, so the next prompt is not masked. There is no second field kind for it.

## Generated passwords

`CreateTenant` returns the passwords it generated for new first admins exactly once. The view keeps them
in memory and draws them under their own heading until the next read of the view or until it is left,
which is how the web's one-time cookie behaves.

## Ending a connection

The server lists only the caller's tenant's connections on the answering server, which may include the
session doing the listing. The confirmation names the surface, the handle and the address, and says that
the holder may reconnect and that ending this session's own connection closes it.
