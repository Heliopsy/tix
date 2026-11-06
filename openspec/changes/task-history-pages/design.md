# Design

## Page size

The task history uses the web handler's page size (`web.page_size`, `core.DefaultDisplayLimit` when
unset), the same number every other browser listing pages by.

## Keeping a move on one row

`groupHistory` folds the hops of one multi-hop move into a row by looking only at adjacent entries.
Paging cuts the sequence, so a cut between two hops would draw the newer hops as one move on one page and
the older hops as another on the next. After reading a page the handler reads one more entry past it:
when that entry chains into the page's oldest, it joins the page and the cursor moves past it, and the
look repeats. Because grouping only depends on adjacent pairs, a page whose boundary falls between two
entries that do not chain groups exactly as the whole trail would. The look stops after 32 hops, far
beyond any workflow path; a longer chain is split there and draws as two rows.

The same extra read answers whether anything is older at all. The service hands back a cursor whenever a
page is full, so a trail that ends exactly on a page boundary would otherwise offer Show older onto an
empty page.

## Without JavaScript

Show older is an `<a>` whose `href` is the task screen with `?history=CURSOR#history`, so a browser
without scripts gets a full page holding that older page with History open, and a Newest changes link.
`hx-get` points at the fragment route instead, and htmx swaps the control's row for the rows plus the
next control. A live update re-renders History from the newest page, dropping rows loaded with Show
older; that is the same reset the rest of the live regions do.

The fragment route is web-only and binds `ListAudit`, which is already bound by the task and activity
screens, so the capability registry and the OpenAPI document do not change.
