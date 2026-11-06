# Task history pages back to the beginning

## Why

The task screen read the task's audit trail with no limit, so it got the store's default page of the 50
newest entries and dropped the cursor. A task with a longer trail showed its latest 50 changes and
nothing said there were more. The terminal's history view had the opposite problem: it read the log in
the listing's default order, oldest first, under a status line saying "newest first".

## What Changes

- The task screen's History draws one page of the newest changes, at the browser's configured page size,
  and a Show older control under the table while older changes exist.
- With JavaScript, Show older fetches the next page as table rows from `GET /tasks/REF/history` and
  appends them in place of itself. Without it, the control is a link to the task screen at that page
  (`?history=CURSOR`), which opens History and offers Newest changes to go back.
- A multi-hop move whose hops would straddle a page boundary is kept whole on the newer page, and the
  last page never offers Show older.
- The terminal's history view reads newest first, as it already said it did.

## Impact

- `internal/web`: task screen, a new web-only fragment route, templates and strings.
- `internal/tui`: the history read's ordering.
- No service, HTTP API or CLI change. `tix audit ls` already reports the next cursor when there is more.
