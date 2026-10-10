# Design

## Truncate: correct, not loud

The brief allowed either making `Truncate` correct for styled input or making it refuse. Correct
wins on two grounds. It is a formatting primitive called from inside eleven render paths, so the
only refusals available are a panic or a second return value: the first turns a cosmetic defect into
a crashed frame, and the second would have to be handled at eleven call sites that are all currently
correct. And the cost of correctness here is one line, because `github.com/charmbracelet/x/ansi` is
already in `go.mod` -- `internal/tui/theme_test.go` imports it -- and `ansi.Truncate(s, width, tail)`
is exactly the operation, escape-aware, with the tail counted against the width.

Measured against the old implementation, plain-text output is identical at every width tested,
including `width == 1`, where the repository's behaviour is to keep the character and drop the mark.
That is the one branch `ansi.Truncate` does not give for free, since a one-cell tail leaves no room,
so it is kept explicit.

No parser was written. Writing one was the alternative the brief warned against.

## The pager: the trail is a floor, the number is the position

`maxTrailDepth` cannot simply be raised -- that moves the stall rather than removing it, and the
bound exists because the trail arrives from the address bar. The count has to be carried.

The invariant is that the page number is at least `len(Trail) + 2` and equals it exactly when no
trimming has happened. So:

- `Page()` is `max(Number, len(Trail)+2)`. A zero `Number`, which is what a pager constructed
  without one has, falls back to the trail's own count, so nothing that reads a pager has to know
  about the parameter.
- `pageFrom` refuses a number below `len(trail)+2`, above `maxPage`, or unparseable. A reader who
  types a *larger* number is believed, and that is deliberate: the count is the link's own
  bookkeeping, the parameter reaches no query, and the existing doc comment already concedes that a
  hand-edited link loses the count. Refusing anything that the trail cannot verify would mean
  refusing every honest value past page 42, which is the whole point.
- `href` writes the parameter only when `page > len(trail)+2` for the destination, so a walk under
  the bound produces byte-identical URLs to before.

`PrevHref` carries `Page()-1`, which is what makes the walk back correct: the trimmed trail cannot
reconstruct the count on its own, and the number can. Previous still runs out after 40 steps and
lands on the cursorless first page, because the oldest cursors were genuinely discarded. That is
trimming's own cost, it was already the behaviour, and the guard asserts it rather than papering
over it.

The parameter is named `page`. The `/columns` POST already has a form field of that name meaning
"which screen's column set", on a different route and a different method; the clash is in the
reader's head rather than in any request, and `page=57` beside `cursor=` and `trail=` is what a
person reading an address bar expects.

## Lease precision: make them agree, at the lease

Documenting the asymmetry was the other option. It loses because the asymmetry is not merely between
two stored rows -- it is between the value returned to the caller and the row it was written to, on
one engine. `Claim.LeaseExpiresAt` is the in-memory `until`; `Claim.Task.LeaseExpiresAt` is read back
from the row. On PostgreSQL those were different instants in the same returned struct. No amount of
documentation makes that defensible.

The cut is at one microsecond and lives in `internal/lease`, whose package comment already says it
"materializes lease expiry". It is not in `internal/store/postgres`, because a truncation there would
fix the row and leave the returned field wrong; and it is not in `internal/clock`, because that would
make every timestamp in the product microsecond-precision on the strength of one engine's column
type -- a bigger claim than the evidence supports, and one that `now.Add(ttl)` could still break for
a sub-microsecond TTL.

The general truncation of every `TIMESTAMPTZ` is documented in the PostgreSQL README, where the
dialect table already lives, because a future value with sub-microsecond meaning will hit it again.

## The service contract: one guard, not four

`core.Service` declares roughly thirty methods as `(*T, error)` and no handler on any surface checks
the pointer. Putting the check in the four handlers the review named would defend four of thirty
against a case that cannot occur, and leave the other twenty-six with the same unstated obligation.

So the obligation is stated on `core.Service` -- a nil error means a non-nil pointer, because an
absent record is an error and a read that succeeded has something to hand back -- and `web.held`
enforces it at the point a read is passed through. A full decorating wrapper over `core.Service`
would be the other way to have one choke point, and would be about eighty methods of boilerplate
over a contract that is explicitly frozen; `held` is six lines and a type parameter.

The guard calls the four handlers directly rather than through the test server, because `net/http`
recovers a panic and answers 500, and the browser interface deliberately renders the same "internal
error" page for a reported fault and a crash. A status code read through a client therefore cannot
tell the two apart, and neither can the body. The adversary is a `core.Service` that returns
`(nil, nil)`, so what is proved is the guarantee rather than the two shipped implementations
happening to satisfy it.

## The oracle stays, in the test file

`matchesText` is called once, by `TestTheActivityFeedShowsEveryMatchItScanned`, to compute the 55 it
expects. Deleting it would let the fixture's own arithmetic stand as the expectation, which is
weaker: the test would then assert that the scan agrees with a number the test author wrote down.
Keeping it in `internal/web/activity.go` ships a function no production path calls. Moving it into
`activity_internal_test.go` gives up nothing -- it is in the same package, so the test reads it
identically -- and puts it where its reason for existing can be written next to it.

## Migration 0002: assessed, documented, not fixed

**Who runs migrations.** The runner executes on the pool connection as the DSN's login role, before
any transaction has entered `tix_app` and with no `tix.tenant_id` set. On a superuser or `BYPASSRLS`
role the policies do not apply. On a plain table owner they do, because `FORCE ROW LEVEL SECURITY` is
on, and `current_setting('tix.tenant_id', true)` is NULL, so the predicate is never true.

**Is a non-superuser deployment supported.** It is the one this repository recommends.
`internal/store/postgres/README.md` says, in as many words, to give the application a plain login
role rather than a superuser, and shows `CREATE DATABASE tix OWNER tix`. So the exposure is not
hypothetical and not limited to an exotic setup.

**Reproduced** on PostgreSQL 18, as a `NOSUPERUSER NOBYPASSRLS` owner of a table carrying the forced
policy: `UPDATE projects SET seq_counter = ...` reports `UPDATE 0` against a table holding one row,
and `SELECT count(*)` from the same session returns 0.

**What it costs today, which is nothing.**

- `0002_project_seq` shipped in v0.1.0, in the same release as `0001_init`. No released database has
  ever had a `projects` row at the moment 0002 applies, so the step is a no-op on every real
  database regardless of the role. Latent as a pattern, unreachable as a fault.
- `0010_token_name_unique` shipped in v0.14.0 and *is* reachable, on an upgrade from v0.13.x or
  earlier. It fails loudly rather than silently: the rename affects no rows, and the
  `CREATE UNIQUE INDEX` in the same transaction then cannot be built, because an index build reads
  the heap and ignores RLS, so it still sees the duplicates. Reproduced:
  `ERROR: could not create unique index "idx_tokens_name_live" / DETAIL: Duplicate keys exist.` The
  transaction rolls back and the upgrade refuses, which is the behaviour wanted even though the
  message points at the wrong cause.

**So what is actually unsafe is the next data migration** -- one whose effect no DDL in the same
transaction depends on. It would apply to zero rows and report success. That means a migration
mutating tenant-scoped rows cannot be reviewed as portable SQL alone until the runner changes, which
is the thing worth writing down.

**Why the fix is not here.** It belongs in `applyMigration`, and every route to it -- lifting
`FORCE ROW LEVEL SECURITY` for the transaction, entering a bypassing role, or driving the data step
once per tenant -- changes the security posture the README's row-level security section rests on, and
needs guards of its own against the obvious failure where the lift outlives the migration. That is a
change of its own size, not a sixth item on a list of small ones.
