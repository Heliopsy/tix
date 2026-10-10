# Design

## Blank clears, absent does not

The service already draws this distinction and the browser was collapsing it. `mergeTaskFields` deletes
every key the update names and re-adds the ones carrying a value, so a key with a null value is the clear
and an omitted key is the no-change. The browser's job is only to preserve which of the two the reader
performed.

The form layer therefore decides on the key's **presence**, not on the value's emptiness. `field(r, name)`
returns the empty string for both an absent key and a blank one, which is exactly the conflation that
produced the defect, so the two call sites that mean "clear this" read `sent(r, name)`, which returns the
value and whether the form carried the key at all.

This also answers the reachability question the fix raises. If blank meant delete and presence were
inferred from the form having been rendered, a partial POST that omitted a field would start deleting it.
It cannot, because omission is not blankness: a caller naming three of seven fields changes three. That
holds for a browser, for a script, and for a future form that renders a subset, without any of them
having to promise completeness. The custom field editor does render an input per definition, so a browser
posting it names every field; that is a property of the screen, not a load-bearing assumption of the rule.

A required field emptied this way reaches `mergeTaskFields` as a named key with no value, the key is
absent from the merged map, and the required check refuses the save. The reader gets the validation error
the service writes rather than a save that reports success and stores nothing.

## An exclusion, not an inclusion

The visibility control records which projects are **hidden** — that is what the cookie has always held,
so that a project created tomorrow appears without being ticked. The task screen then inverted it into a
list of projects to show and handed that to the store.

Inverting was the defect. It made the listing depend on the screen having been able to enumerate every
project, and at the design target — a million tasks, and no stated bound on projects — it would put one
term per project into every task query. Passing the hidden keys straight through as
`TaskFilter.Exclude.ProjectKeys`, which both engines already answer, is the same answer at any size: the
filter names what the reader put away, which is small by construction and comes from the reader's own
preference rather than from a truncated listing.

`MaxPageLimit` was rejected as the fix. It moves the cliff from fifty to five hundred and leaves the same
shape of failure behind it, silent in the same way.

## Walking the listing, where the whole set is the point

The filter no longer needs the listing, but four other things still do: the boxes the control offers, the
accent per project, the workflow per project, and the tenant diagram's count. Each of those is wrong,
silently, on a page of the set, so `allProjects` walks the cursor at `MaxPageLimit` a page.

It is bounded, at forty pages, because an unbounded walk inside a request handler is how a request stops
returning. The bound is twenty thousand projects, which is far past any tenant this is built for, and
`allProjects` reports whether it finished so a caller that prints a figure can say "unavailable" rather
than print a number that is short. The tenant diagram is the only such caller; every other row there
already had that fallback and the project row did not.

Projects are administrative records, orders of magnitude fewer than tasks. Walking them is not the same
proposition as walking a task table, and nothing here walks a task table.

## The free-text box belongs to the store

A page assembled partly by the store and partly by the caller cannot report a cursor. The store's cursor
names the end of the store page; the caller's page ends wherever it stopped keeping rows. The gap between
those two is exactly the set of matches that were skipped.

Building the right cursor in `internal/web` was rejected: `auditCursor` is unexported in
`internal/service` for a reason, and reconstructing its shape in the web package would put keyset
knowledge in a presentation layer, against the architecture invariants.

Making the predicate part of the filter removes the problem rather than patching it. `AuditFilter.Text` is
a weak, case-insensitive substring match over what an entry records — its action, subject kind, source and
both snapshots — which is the same set of haystacks the screen's own matcher used, so the question the box
asks does not change. Both engines answer it through one shared predicate in `internal/store/sql`, beside
the one that already answers explicit title and body terms, and each dialect names its own columns because
the snapshots are JSONB on PostgreSQL and TEXT on SQLite. The case folding carries the same ASCII-only
residue that predicate already documents.

Two consequences follow, and both are improvements. The search now covers the whole log rather than the
eight store pages the scan was bounded to, so a match older than four hundred entries is findable at all.
And the screen can no longer say "out of the N most recent records searched", because there is no window
— that sentence, and the field behind it, go.

## The delivery pager

Nothing to design. The handler already computed the cursor; `newPager` and the `pager` partial are how
every other keyset listing on this surface renders one. The screen's other table, the endpoint list, is
not paginated, so the one control belongs to the log.
