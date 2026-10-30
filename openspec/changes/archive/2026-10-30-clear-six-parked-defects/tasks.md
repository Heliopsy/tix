# Tasks

## 1. Truncate measures what it draws

- [x] 1.1 `internal/tui/layout.go`: `Truncate` is `ansi.Truncate` from
      `github.com/charmbracelet/x/ansi`, which the repository already depends on, keeping the
      one-cell branch where the character wins over the mark
- [x] 1.2 `internal/tui/width_test.go`: `TestTruncateMeasuresStyledTextByWhatItDraws`, asserting the
      drawn width and the visible text rather than bytes, so it holds for any escape-aware cut
- [x] 1.3 `internal/tui/width_test.go`: `TestTruncateToOneCellKeepsTheCharacterNotTheMark`, the
      branch the ellipsis path does not cover

## 2. The page number counts past the trail's bound

- [x] 2.1 `internal/web/pager.go`: `PageParam` and `maxPage`, and `pageFrom`, which refuses a number
      below what the trail already proves
- [x] 2.2 `internal/web/pager.go`: `pager.Number`, and `Page` reading it as the position with the
      trail as the floor
- [x] 2.3 `internal/web/pager.go`: `href` takes the destination's page and writes the parameter only
      where that destination's trail cannot count it; `NextHref` and `PrevHref` carry it
- [x] 2.4 `internal/web/pager_internal_test.go`:
      `TestThePageNumberKeepsCountingPastTheTrailsBound`, walking through the control's own links in
      both directions, since the defect was that the links stopped carrying enough to count with
- [x] 2.5 `internal/web/pager_internal_test.go`:
      `TestThePageNumberIsWrittenOnlyWhereTheTrailCannotCount`, the typed values and the early pages

## 3. A lease expiry is one instant on both engines

- [x] 3.1 `internal/lease/lease.go`: `Precision` and `Until`, in the package whose own comment says
      it materializes lease expiry
- [x] 3.2 `internal/service/claim.go`: the four places that computed `now.Add(ttl)` call it --
      `ClaimTask`, `ClaimNext`, `applyWorkflowTTL` and `RenewLease`
- [x] 3.3 `internal/store/postgres/README.md`: the general `TIMESTAMPTZ` truncation, in the dialect
      section where the next sub-microsecond value will be looked up
- [x] 3.4 `internal/lease/lease_test.go`: `TestUntilIsCutToWhatEveryEngineKeeps`, including that it
      truncates rather than rounds
- [x] 3.5 `internal/service/claim_test.go`: `TestALeaseExpiryIsCutToWhatEveryEngineStores`, from a
      clock deliberately set off a microsecond boundary, over all three paths that take a lease
- [x] 3.6 `internal/store/postgres/claim_test.go`:
      `TestALeaseExpiryAtLeasePrecisionSurvivesTheRow`, which asserts both halves: the column
      truncates nanoseconds, and an expiry at `lease.Precision` survives it

## 4. The service's pointer contract is stated and guarded once

- [x] 4.1 `internal/core/service.go`: the `Service` doc states that a nil error means a non-nil
      pointer, and why
- [x] 4.2 `internal/web/render.go`: `held`, the one choke point
- [x] 4.3 `internal/web/status.go`, `stats.go`, `workflows.go`, `connections.go`: each read passes
      through it
- [x] 4.4 `internal/web/contract_internal_test.go`: `answersNothing`, a service that breaks the
      guarantee, and `TestAHandlerReportsAServiceThatAnswersWithNothing`, calling the handlers
      directly because `net/http` recovers a panic into the same 500 the reported fault produces

## 5. Dead code

- [x] 5.1 `internal/web/render.go`: `branding.Monogram` deleted, with both assignments
- [x] 5.2 `internal/web/activity.go`, `internal/web/activity_internal_test.go`: `matchesText` moved
      into the test file, where the reason it is kept can be written beside it

## 6. Migration 0002's row-level security exposure

- [x] 6.1 Assessed: reachable in the deployment this repository recommends, reproduced on PostgreSQL
      18, and harmful to no shipped migration. See `design.md`
- [x] 6.2 `internal/store/postgres/README.md`: "Migrations run outside the policies they create" --
      the mechanism, the reproduction, what each of 0002 and 0010 would cost, and why the fix belongs
      in the runner
- [x] 6.3 `internal/store/postgres/schema.go`: `runMigrations` points at it
- [x] 6.4 Deliberately not done: the runner change itself, which alters the security posture the
      row-level security section rests on and is a change of its own size
