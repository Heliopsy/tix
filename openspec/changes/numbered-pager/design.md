# Design: numbered pager

## The total

A task listing is counted by `store.Tx.CountTasks`, which applies exactly the predicates `ListTasks`
applies (`applyTaskFilter`, the tenant-scoped builder, the same transaction, so PostgreSQL row-level
security scopes it as well) and renders

    SELECT COUNT(*) FROM (SELECT 1 FROM tasks JOIN projects ... WHERE ... LIMIT 10001) AS counted

The inner `LIMIT` is the cap. On both engines the count stops reading at the cap, so a tenant with a
million tasks pays for ten thousand index entries, not a million. A count of 10,001 is reported as
10,000 with `TotalCapped`, and every surface writes it as a floor ("400+", `total_capped: true`).

No count already existed for a filtered listing: the view counts (`WorkCounts`, `CountTasksInStates`)
answer fixed questions, not an arbitrary filter, so a new store method was needed.

The count runs on every `ListTasks` call. It is bounded by the cap and costs no more than the sort the
page itself already does over the same selection, and an agent that never reads `total` pays one
indexed count per page.

## Jumping: a seek, not OFFSET

`AGENTS.md` forbids `OFFSET`, and the reason holds: every page reached through the API or by Next is
a keyset page that neither skips nor repeats under concurrent writes. So page N is translated to a
cursor. `store.Tx.SeekTask(f, n)` walks the same ordering, with the same keyset predicate and star
rank, `LIMIT n`, and returns the n-th row; the service encodes that row as the cursor and reads the
page from it with the ordinary keyset query. The seek reads at most the capped count of rows, which is
the price `OFFSET` would pay too, but the page itself is still keyset-addressed.

A page past the end is clamped to the last page, and the response says which page it answered.

`PageNumber` and `Page.Cursor` are exclusive on the contract. The cursor is unchanged and remains the
API's primary mechanism; `page` is for a person who wants page 12.

## Other listings

The other paged listings (actors, users, tenants, tenant users, projects, inbox, activity, sync runs,
deliveries, channel deliveries, account log) return `(rows, next, err)` on the contract, and giving
each a count of its own would change eleven service methods, the HTTP routes, the client and MCP for
lists that are small in practice. Instead the web counts them by walking their own listing in 500-row
keyset stretches, up to 1,000 rows (`walkCountCap`), and seeks page N the same way. Past the cap they
read "Page 2 of 20+". Their API responses are unchanged.

## The address

- `page=N` with no `cursor` is a position. The pager's own links, the numbered links and the go-to
  form all write `page=N` only, so a page is a short, shareable link.
- A link that carries `cursor` is a cursor walk, as before; `trail` and the old label-only `page`
  still count it. Next from beyond the counted range falls back to a cursor link.
- The view switch keeps `page` and drops `cursor` and `trail`; the page may hold different tasks on the
  board, since the list narrows to open work, but the reader stays on the page number they chose.
- The filter form submits no `page`, so Apply resets to page 1, and Clear, a project narrowing link
  and Show all drop it for the same reason.
- The lanes grouping form keeps the page: changing what a lane is does not change the selection.

## Live updates

The live refresh fetches the screen's own address, so a reader on page 4 is redrawn on page 4. With a
seek, an insert before page 4 shifts its rows by one, the same as any numbered pager; the reader is not
moved and the summary and pager are redrawn with the new total. A page that no longer exists after
deletions is clamped to the new last page. A cursor-addressed page keeps its exact keyset behaviour.

## The terminal

The board reads up to 500 tasks per load. When the total says there are more, the status bar says
"page N/M" and `pgdown`/`pgup` read the next and previous 500 by page number. It does not offer a typed
jump: 500-task pages make a project that needs one rare, and the palette offers the page keys.

## Trade-offs

- A numbered page is not a snapshot. Two readers on page 3 a minute apart may see a shifted row.
  Cursor links are still exact, and the API's cursor is unchanged.
- A count costs one capped query per task listing.
- The walked count for other listings costs up to two extra 500-row reads on a long log.
