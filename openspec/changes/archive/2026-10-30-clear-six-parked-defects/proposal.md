# Clear six defects a review parked as low severity

## Why

None of these was urgent and all of them were real. They were found by review, labelled low severity
and left, which is the right call once and the wrong one indefinitely: four of the six are traps for
the next person to touch the code rather than faults a reader can hit today, and a trap costs more
the longer it sits because the context that explains it goes away.

- **`tui.Truncate` counted escape bytes as content.** It measures terminal cells correctly for plain
  text, which was fixed earlier, but `Truncate("\x1b[31mhello\x1b[0m", 6)` returned `"\x1b[31m…"`:
  the whole visible word destroyed at a width it fits in twice over, and the terminal left holding
  an unterminated colour. Every current caller is handed plain text and styles afterwards, which is
  the only reason it is latent.
- **The pager's page number stalled past 41 pages.** `Page()` was `len(Trail) + 2` and the trail
  stops growing at `maxTrailDepth`, so page 42 and every page after it reported "Page 42". The one
  number on the control that says where the reader is froze while they kept walking.
- **A lease expiry was two different instants depending on the engine.** The service computed it at
  the nanosecond the clock reported. PostgreSQL's `TIMESTAMPTZ` truncated what it stored while the
  holder was handed the untruncated value, so the row freed the lease up to 999ns before the moment
  its holder was told, and `Claim.LeaseExpiresAt` disagreed with `Claim.Task.LeaseExpiresAt` on that
  engine alone. SQLite kept the nanoseconds. Sub-microsecond, so nothing observable turns on it --
  but two engines holding different instants for one claim is what makes a later bug unexplainable.
- **Four web handlers dereferenced a service pointer having checked only the error.**
  `/admin/status` is the worst of them: `StatusReport.AttachedCount` has a value receiver, so the
  dereference is implicit and the panic lands before any field is named. Both shipped
  implementations return non-nil on success -- `service.Local` by construction, `client.Client`
  because `call[Out]` allocates even for a 204 -- so a previous reviewer could not build a failing
  case and marked it unproven.
- **Two pieces of dead code.** `branding.Monogram` is referenced by no template; the only monogram
  in use is `userRow.Monogram`. `matchesText` is called only by a test using it as an independent
  oracle, after the audit text filter moved into the store.
- **Migration `0002` mutates data under a row-level security policy no migration sets.** `0010`
  inherited the same exposure. This one is assessed and documented rather than fixed; see below.

## What Changes

- **`Truncate` is escape-aware.** It is `ansi.Truncate` from `github.com/charmbracelet/x/ansi`,
  already a dependency of this repository, rather than a hand-rolled rune loop. Correct for styled
  input was chosen over refusing it: a refusal would be a panic or an error return in a formatting
  primitive called from inside render paths, and the library that already measures cells for us
  also carries escapes through a cut. Behaviour for plain text is unchanged at every width,
  including the one-cell case where the character wins over the mark.
- **The pager carries its page number past the point the trail can count.** A bounded `page`
  parameter sits beside the trail, written only once trimming has actually lost something, so no
  URL short of 43 pages grows a parameter. The trail is the floor and the number is the position, so
  a hand-edited value cannot report fewer pages than the trail proves and cannot reach a query --
  the worst it costs is a wrong label, exactly as for the trail itself.
- **A lease expiry is cut to `lease.Precision` before it is stored or reported.** One microsecond,
  the coarsest resolution any shipped engine keeps. `lease.Until` materializes it, which is what the
  `lease` package's own doc comment already says it is for, and the four places that computed
  `now.Add(ttl)` call it. Truncation, never rounding: an expiry moved later than the instant it was
  computed for would outlive what the holder was told.
- **`core.Service`'s pointer contract is stated, and enforced once.** A method declared
  `(*T, error)` returns a non-nil pointer whenever it returns a nil error. `web.held` passes a read
  through and turns the nil pair into the fault it is, so the obligation lives on the contract and at
  one choke point instead of being defended in four handlers for a case none of them can produce.
- **Both pieces of dead code are gone.** `branding.Monogram` is deleted. `matchesText` moved into
  `activity_internal_test.go`: the oracle is worth keeping -- a fixture counted by the code under
  test proves only that the code agrees with itself -- but it does not belong in the shipped
  package, and the test file is where a reader looking for why it exists will be.
- **Migration 0002's exposure is documented, not fixed.** See `design.md`: it is reachable in the
  deployment this repository recommends, no shipped migration is actually harmed by it, and the fix
  belongs in the PostgreSQL migration runner, which is a larger change than this one.

## Impact

- `internal/tui/layout.go`: `Truncate`. Every caller is unaffected; `charmbracelet/x/ansi` moves
  from a test-only direct dependency to a production one, with no change to `go.mod`.
- `internal/web`: `pager.go` (`PageParam`, `pager.Number`, `Page`, `PrevHref`, `NextHref`, `href`),
  `render.go` (`held`, and `branding` loses a field), `status.go`, `stats.go`, `workflows.go`,
  `connections.go`, `activity.go`.
- `internal/core/service.go`: the `Service` doc comment states the pointer guarantee. No signature
  changes, so no implementation has to change.
- `internal/lease/lease.go`: `Precision` and `Until`. `internal/service/claim.go` calls it.
- `internal/store/postgres/README.md` and `schema.go`: the timestamp precision and the migration
  exposure.
- One new URL parameter, on listing screens only, past page 42. One behaviour change a reader could
  notice: a lease expiry now ends in a whole microsecond.
