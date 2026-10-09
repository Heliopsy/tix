# Design

## Where the refusal's field attribution lives

A form that re-renders itself has to know which control a refusal concerns. Two places could decide:
the presentation layer, by matching the message text, or the rule itself.

Matching text at the presentation layer means the message and its placement drift apart the first time
anybody rewords the message, and it means the HTTP API learns nothing. `core.Error` already carries
`Details`, and `checkScopeGrant` already uses it to name the offending scope, so the rule names its field
there: `core.DetailField` is the key, `core.FieldOf` reads it. The browser places the message; the
service decides what it is about. A refusal naming no field renders at the top of the form.

## Why a refused form returns its own status rather than 200

`layout.html` sets `hx-boost="true"`, so every form on every screen is submitted by htmx, and htmx does
not swap a non-2xx response. `live.js` already forces it to (`shouldSwap = true` for any status at or
above 400), with the note that "every refusal here renders a full page explaining itself". A re-rendered
form is such a page, so it keeps the refusal's own status — 400 for a validation failure, 409 for a
conflict — and htmx swaps it in. With scripting off, the browser renders the same body for the same
status. One response serves both paths, and neither depends on the other.

`correctable` decides which refusals re-render: invalid, conflict and precondition. Everything else goes
to the error screen, because a lost session or a missing scope is not something editing the form fixes,
and the error screen is the only place that explains it.

## The scope of token name uniqueness

Per tenant, among tokens that are not revoked.

Per tenant because every credential here is tenant-owned, and `ssh_keys` already sets the precedent with
`UNIQUE (tenant_id, fingerprint)` and the note saying why. Wider would let one tenant discover, and
constrain, what another has named its credentials.

Among live tokens because the defect is ambiguous revocation, and a revoked token is not a revocation
candidate. It also makes rotation work: revoke `ci`, mint `ci`. Holding a name forever would force every
rotation to invent `ci-2`, which reintroduces the problem it was meant to solve.

Expiry deliberately does not free a name. An expired token is still listed, still distinguishable, and
still a thing an operator may want to revoke properly; more practically, "expired" is a function of the
current time and cannot be an index predicate, so a service rule keyed on it would disagree with the
database under it.

## Why both a service check and a database index

The service check is what produces a message an operator can read and a field a form can place it
against. It runs inside the same transaction as the insert.

That is sufficient on SQLite, which has one writer. It is not sufficient on PostgreSQL: transactions run
at read committed, so two creations racing both see the name free and both insert. The partial unique
index closes that, and it also covers any future writer that forgets the rule. Its refusal is not a
usable message — it names a constraint — which is why the service check stays in front of it rather than
being replaced by it.

## The upgrade path for existing duplicates

`0010` renames the duplicates it finds before creating the index. The alternatives were worse: revoking
all but one stops an agent that is working right now in order to tidy a listing, and letting the index
creation fail leaves an operator with a migration that will not apply and no instruction.

The new name appends the row's identifier, which is the primary key, so the rename cannot itself collide
and the resulting name says exactly which row to look at. The row left untouched is the lowest
identifier, which for a ULID is the oldest, so a long-standing token keeps the name people refer to it
by.

**Known limitation worth a reviewer's attention.** On PostgreSQL every tenant-scoped table has
`FORCE ROW LEVEL SECURITY`, with a policy keyed on the `tix.tenant_id` setting, which a migration does
not set. A migration connection that is a superuser or holds `BYPASSRLS` — which is the ordinary case,
including the project's own test container and any deployment where tix owns its database — bypasses the
policy and the rename applies. A connection that is merely the table owner would see no rows, the rename
would be a no-op, and the index creation would then fail loudly on a database that holds duplicates. This
is not new with this change: `0002`'s `UPDATE projects SET seq_counter = ...` has the same exposure and
has had it since the schema's second migration. Fixing it properly means giving the Postgres path a way
to run a data statement with the policy lifted, which is a change to the migration runner rather than to
this screen.

## Why the expiry control offers durations rather than a date

"90 days" is the decision an operator is making. A datetime field makes them do the arithmetic, in a zone
neither side has agreed on, and a date alone silently means midnight somewhere. The command line already
takes an absolute date, so both grammars exist on the surfaces that suit them and neither has to parse
the other's.

The control is a closed list, and the handler accepts only the keys the list declares, so a value typed
into the request cannot reach the service as an unbounded expiry or be read as no expiry at all.

## Why the proposed default is an expiry rather than none

Before this change the browser could not express an expiry, so every token it minted never expired — by
omission, not by anybody's decision. Proposing 90 days therefore overrides no reader's choice; it changes
what happens when a reader does not choose, in the direction that bounds the damage of a leak. "Never
expires" stays one option away, named, so a credential that genuinely needs it is still one selection.

The command line's default is unchanged: `tix token create` with no `--expires` still mints a token that
never expires. The two surfaces disagree, deliberately — a flag that was explicitly omitted is a
decision, a form control that was never touched is not — and that is the thing in this change most worth
arguing about.
