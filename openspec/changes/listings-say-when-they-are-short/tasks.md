# Tasks

## 1. The tenant diagram's token figure

- [x] 1.1 `internal/web/admin.go`: `tokenShapeRow`, building the API tokens row from `tokenListing`
      so the figure is the token screen's own rows counted
- [x] 1.3 `internal/web/admin.go`: `countShort`, what a row prints where its walk stopped, beside
      `countFailed`, which is a read that errored

## 2. The actor directory is walked

- [x] 2.1 `internal/web/admin.go`: `allActors` and `actorScanPages`, the cursor walk `allProjects`
      already established, replacing one page of `core.MaxPageLimit` with the cursor discarded
- [x] 2.2 `internal/web/admin.go`: `tokenList`, so the listing carries whether it reached every
      actor rather than returning two unnamed booleans
- [x] 2.3 `internal/web/templates/tokens.html`: the table states when the rows are not every token
      of the tenant

## 3. The statistics picker

- [x] 3.1 `internal/web/stats.go`: the picker is built from `allProjects`, keeping the decision that
      a failed read costs the picker and not the figures
- [x] 3.2 `internal/web/templates/stats.html`: the screen states when the picker does not list every
      project

## 4. One caveat per control, not per screen

- [x] 4.1 `internal/web/prefs.go`: `visibilityFormView.Whole`, and `prefsFor` takes the walk's
      completeness
- [x] 4.2 `internal/web/templates/partials.html`: the shared control states its own shortness
- [x] 4.3 `internal/web/session.go`, `internal/web/tasks.go`: `settingsView.ProjectScan` is gone and
      the task screen passes the flag it was dropping
- [x] 4.4 `internal/web/admin.go`: a short project walk prints `incomplete` rather than
      `unavailable`, which is a read that failed

## 5. Guards

- [x] 5.1 `internal/web/fixture_test.go`: `actorPagesNeverEnd` and `projectPagesNeverEnd`, a listing
      that answers with another cursor forever, since the real short case is 20,000 rows
- [x] 5.2 `internal/web/listings_test.go`: the tenant figure equals the token screen's row count for
      an administrator, and is not the reader's own
- [x] 5.3 `internal/web/listings_test.go`: `seedActorsPastAPage`, and a token held past the first
      directory page reaches the listing with its revocation control
- [x] 5.4 `internal/web/listings_test.go`: the statistics picker offers a project past the first page
      and keeps it selected
- [x] 5.5 `internal/web/listings_test.go`: every screen that can stop short says so, and says nothing
      when the walk reached the end
