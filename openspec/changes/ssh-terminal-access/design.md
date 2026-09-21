# Design

## The seams this rides on

`tui.Options` and `tui.Config` already take the streams the interface reads and writes, so a session's streams
can be handed to it directly. `internal/service` takes its tenant from the actor in each call's context, never
from state on the service, so one service instance serves many tenants concurrently without a per-session
instance. `golang.org/x/crypto/ssh` was already a direct dependency.

## Why a tenant per fingerprint rather than a shared board

A shared demo board shows visitors working alongside each other, which is a better story, but it needs a capped
role, a global reseed on a timer, and an answer for every way one visitor can spoil another's afternoon.

A tenant per fingerprint is smaller and stronger. A visitor alone in their sandbox can be given real authority,
including the screens a member cannot reach, because there is nothing of anyone else's to break. Seeding happens
once, at creation, instead of resetting a shared tenant on a timer. And the demo now exercises the real
isolation machinery on every connection: two visitors are two tenants, so the scoped builder, the tenant
predicate and, on PostgreSQL, row-level security are all doing their actual job in front of an audience.

## Why the tenant key is derived from the fingerprint

The mapping from a fingerprint to its tenant has to survive a restart, and it has to make a return visit a
lookup rather than a fresh seed. Deriving the tenant key as `sandbox-` plus a digest of the fingerprint makes
the mapping a pure function of the key the client proved: there is no second table to keep consistent, no row
that can be orphaned, and `GetTenantByKey` is the whole lookup. The prefix also bounds what this listener will
ever count, touch or delete, so pointing it at a database holding other tenants cannot destroy them.

`tenants.updated_at` carries the last-seen time. A connection touches it, and the reaper selects on it, so the
life slides from the last visit rather than from creation. Counting from creation would delete the sandbox of
someone actively using it, which is the opposite of what the mapping is for.

## Why the deletion is the store's hard delete

`Service.DeleteTenant` is a soft delete: it marks the tenant and leaves every row in place. That is right for a
real tenant and wrong here, where sandboxes arrive continuously and the rows would accumulate forever. The
reaper calls `UnscopedTx.DeleteTenant`, which removes the row; every tenant-owned table declares
`REFERENCES tenants(id) ON DELETE CASCADE`, and SQLite is opened with `foreign_keys` on, so the cascade is
complete on both engines. A test counts rows in four tables after a reap rather than trusting that.

## Why the caps are here and not in the service

A ceiling on tenants and on tasks per tenant is a property of this listener, not of tix: no other caller wants
one. The task cap is a wrapper implementing `core.Service`, overriding the one method that grows the database
and delegating the rest, so it cannot accidentally widen anything. Its count comes from the same tenant-scoped
listing every other reader uses, asking for exactly the limit, so it can never see another sandbox's rows.

When the tenant cap is reached, a **new** fingerprint is refused with a message that says so. An existing
sandbox is never evicted to make room. The cap now bites against tenants that are alive because people keep
returning, and silently deleting somebody's work to admit a stranger would be the worst behaviour available.

## Session isolation

Every connection builds its own actor, its own context and its own model. Nothing is shared but the service and
the store beneath it, and both take their tenant from the context they are handed. That is proved rather than
assumed: a test runs concurrent sessions under the race detector and asserts that no session sees another's
tasks through a listing, and that naming another tenant's task by its identifier returns not-found rather than
the task.

## Rate limiting

A token bucket per source address, in the package, on the standard library. `wish/ratelimiter` would have done
it at the cost of two more modules and a message this listener does not control. Buckets that have refilled
carry no state worth keeping, so the map is bounded by dropping them.

## Deciding on colour

`tui.ColorEnabled` ends in a character-device test, which a network stream can never pass, so over SSH the
answer has to be worked out rather than measured. `tui.Config` takes an explicit `Color *bool` for exactly this,
and the listener supplies one derived from what the client itself said in the session environment: either
`NO_COLOR` spelling set to a non-empty value opts out, `TERM=dumb` opts out, and a session that named no
terminal type at all opts out, because nothing is left to catch the assumption if it is wrong. Everything else
gets colour. A pseudo-terminal request always carries a terminal type and the session appends it last, so it
wins over anything a client set with an env request.

`Config.Out` is not set at all for a session: the program's output is the session, and with the colour decision
made explicitly there is nothing left that wants to inspect a file.

There is a second gate behind that one, and it is easy to miss because the first hides it. lipgloss resolves its
colour profile once, for the whole process, from that process's own standard output. A server's output is a log
file or a journal, so the profile lands on Ascii and every style the interface builds is stripped on its way
out, whatever the theme decided. The listener pins the default profile to ANSI-16, which is exactly the depth
the interface's palette uses and the depth any colour terminal understands. The two gates compose correctly: a
client that opted out gets a theme carrying no colour at all, so the pinned profile has nothing to emit for it.

Pinning a package-global is not free, and the honest fix is a renderer per session: `wish/bubbletea` offers
`MakeRenderer(sess)`, which resolves the profile from that client's own terminal, and `tui.Config` would need to
accept a `*lipgloss.Renderer` for the theme to be built from it. That would also let two concurrent sessions
render at different depths, which a process-wide profile cannot. Until then the profile is pinned at the
shallowest depth that renders this palette faithfully, so the imprecision costs nothing visible.

## What this had to work around

The interface has no place to put a line of chrome, so the notice that this is a sandbox and how long it
survives is carried by the first seeded task and printed again when the session ends and the alternate screen
is gone.
