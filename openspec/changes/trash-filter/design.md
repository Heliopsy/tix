# Design

## A tri-state, not a second boolean

`IncludeDeleted` is a boolean that widens. The new question narrows, and it has a useful negation,
so it is a `TriState` like `Claimed` or `Starred`:

| `Trashed` | `IncludeDeleted` | Listed |
| --- | --- | --- |
| Either | false | live tasks (the default) |
| Either | true | live and deleted |
| Yes | any | deleted only |
| No | any | live only |

`Trashed` decides whenever it is set. The alternative, refusing `is:trashed -is:deleted` as a
contradiction, buys nothing: every combination has one obvious answer, and a refused filter in the
browser is a message the reader has to work around. `is:deleted -is:trashed` reads as "live only",
which is what it answers.

Both stores translate it to one predicate on `tasks.deleted_at` in place of the existing
`deleted_at IS NULL` line, so the keyset ordering and cursor are untouched: the trash is the same
ordered listing with a different `WHERE`.

## Surfaces

- Parser: `is:trashed` and `-is:trashed` only. No `trashed:true` boolean spelling, since that form
  exists for links written before `is:` and no such link can name the trash.
- CLI: `--trashed`, so `TestTheCommandLineOffersAFlagForEveryFilterItShould` needs no exemption.
  There is no `--untrashed`; `-is:trashed` is the default listing already.
- HTTP: `trashed=true|false`, read and written by the same tri-state helpers as `claimed`.
- TUI: nothing beyond the parser and `query.Matches`. Its restore refusal now names `is:trashed`.

## The web trash

A sidebar entry, under Tasks, linking to `/tasks?q=is:trashed`. It is a place rather than a way of
drawing the list, so it belongs in the navigation rather than in the View menu, and a link means the
trash is bookmarkable and still just a filter in the box. Tasks is not marked current while the
trash is.

The trash listing:

- is titled Trash, and its summary counts deleted tasks instead of open and done ones;
- is always drawn as the list, whatever the reader chose for the task screen. The board's columns
  are workflow states a deleted card cannot move between, and lanes offer the same moves;
- skips the open-work default, since a deleted task's status is not what makes it worth finding;
- leaves out the quick-add row, which would add live tasks to a screen that does not show them.

A deleted row, on the trash or on an `is:deleted` listing, carries a "deleted" marker with the time,
shows its status and priority as plain labels instead of menus, has no tick, and offers Restore. Restore
posts to the existing `/tasks/{ref}/restore` with `next` set to the listing, so the reader lands
back on the trash with the "task restored" toast and the row gone.
