# A listing that read part of itself says so

## Why

Four screens in the browser interface presented a part of a listing as the whole of it. They share
one shape with the defect fixed earlier in `allProjects`, where hiding one project removed every task
in projects past the fiftieth: a figure or a control was built from one page of a keyset-paginated
listing, and nothing on the screen said so.

- **The tenant diagram's "API tokens" figure was the reader's own count.** `tenantShape` called
  `ListTokens(ctx, "")`, and an empty actor has always meant the caller. The token screen now shows a
  tenant administrator every token of the tenant, so for an administrator the figure disagreed with
  the screen it links to, while sitting between Members, Domains and Workflows, which are all
  tenant-wide.
- **The admin token listing read one page of actors and dropped the cursor.** `core.Page{Limit:
  core.MaxPageLimit}` is 500 actors, and `NextCursor` went on the floor, so a tenant with more actors
  than that lost the tail of its credentials off the one screen an operator reads during an incident.
  The token they came to revoke was simply absent.
- **The statistics project picker read `core.DefaultPageLimit`.** A tenant past fifty projects could
  not filter the statistics by its fifty-first. The figures were never wrong; the control did not
  offer the project.
- **A walk that stops at its bound was silent on three screens.** `allProjects` reports whether it
  finished, and only the settings screen said so. The same control on the task screen, the statistics
  picker and the tenant diagram's figures did not.

## What Changes

- **The tenant diagram counts the listing the token screen renders.** The "API tokens" row is built
  from `tokenListing`, the same function `/admin/tokens` draws its table from, so the figure and the
  screen it links to agree by construction rather than by both being maintained. The tenant screen is
  gated on the scope that listing branches on, so the figure there is always the tenant's; a reader
  who holds that scope without `token:admin` cannot list tokens at all, and the row says the figure
  is unavailable rather than printing one.
- **The actor directory is walked, not sampled.** `allActors` walks the cursor at
  `core.MaxPageLimit` a page, bounded by `actorScanPages`, and reports whether it finished, following
  `allProjects` exactly. The bound exists because an unbounded walk inside a request handler is how a
  request stops returning.
- **The statistics picker offers every project**, through the same `allProjects` walk the task screen
  and settings already use. A failure to read it still leaves the screen without a picker rather than
  without numbers, and now says that rather than rendering a short list silently.
- **Every screen whose listing can stop short says so, in the register the rest of the interface
  uses.** The project visibility control carries its own shortness, so the task screen and the
  settings screen say it from one partial instead of one of them holding the caveat; the token table
  says when it could not reach every actor; and a diagram row whose walk stopped prints `incomplete`
  in the figure's place rather than a floor presented as a total.
- **`projectScanPages` stays at 40.** 20,000 projects is the bound, and it is what stops a request
  walking an unbounded table. Raising it trades a cliff that is now visible on every screen for a
  handler that may not return; the consumers were the problem, and they now handle the short case.

## Impact

- `internal/web`: `admin.go`, `stats.go`, `prefs.go`, `session.go`, `tasks.go`,
  `templates/tokens.html`, `templates/stats.html`, `templates/partials.html`,
  `templates/settings.html`.
- No new route, no service operation, no capability entry: every listing involved already existed and
  every call was already one the reader was allowed to make.
- Observable change in one figure's meaning: for a reader holding `tenant:admin`, the tenant
  diagram's "API tokens" count is the tenant's rather than their own.
- Cost. The tenant screen now pays what the token screen pays for an administrator: one actor
  directory walk plus one `ListTokens` per actor, because the service's token contract is per-actor on
  every surface. That is bounded by a tenant's staff and agents, not by its tasks, and neither screen
  is on a hot path. The statistics screen's picker goes from one query of fifty rows to one query of
  five hundred for any tenant under that, and a walk beyond it. No listing here is the task table.

Not changed, and deliberately: no tenant-wide token count is added to `internal/service`. A count
query would be cheaper on the tenant screen than the walk, and it would also be a second answer to
"how many tokens does this tenant have" that could drift from the listing, which is the defect being
fixed.
