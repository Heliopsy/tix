# Design: compact filter toolbar

## One form, several popovers

The toolbar is `div.tbar` holding the filter form (`form#filterbar.filterbar`, GET, action the
screen's own route) and the buttons after it. The form is `display: contents`, so its box and the
buttons beside it are one flex row. Filters ▾ is a `[popover]` inside the form, so its pickers are
the form's fields. Sort ▾ and ⋯ are popovers outside it, because they also hold forms of their own
(the list grouping posts to `/listgroup`, the columns and project visibility post to their handlers,
Group lanes by is its own GET form), and a form may not nest. Their fields that belong to the filter
(sort, direction, page size) name it with `form="filterbar"`.

Every popover opens from a `popovertarget` button, so it works without scripting, and Escape and a
press outside close it with nothing of ours. No `<details>`.

## Picked terms are folded into the expression

The box is the canonical filter: chips, the saved filter, the pager and the live refresh all read
`q`. A picker cannot edit `q` without a script, so the Filters popover submits `pick_status`,
`pick_priority`, `pick_assignee` and `pick_tag`, and beside each of the first three the value it was
drawn with (`was_KEY`). The task list and the project page fold a request carrying any `pick_`
parameter and answer `303` to the same route with the new `q`, without the `pick_`/`was_` fields and
without the page position, so a change of filter starts at page one and the address stays shareable.

- A picker whose value equals its `was_` value changed nothing and is ignored. That keeps a term
  typed into the box in the same submission from being overwritten by the picker's stale value.
- A changed status, priority or assignee replaces every term of its key that selects (negated terms
  are left alone); the empty choice removes them.
- A tag picker adds `tag:VALUE` unless the expression already selects it.
- When the expression holds several selecting terms of a key, the select shows "as typed" (`*`),
  which leaves them alone.

The deadline window (`due`), the project control (`project`) and the open/all toggle (`all`) were
already parameters of their own and stay so. With scripting the box still narrows the listing as it
is typed; feel.js now leaves out pickers that did not change, so live typing never redirects, and it
redraws the chips and the Filters count with the listing. The popovers apply on Apply.

## Chips

`filterChips` lists, in order: the due window, each project key of `project` (not on a project
page), each term of `q` in the order typed, and `all=1`. A term chip removes exactly its own raw
token from `q`, so quoting and negation survive in the others. Every chip link and Clear all keep
the sort, direction, view, lanes grouping, page size and select mode, and drop the cursor, its trail
and the page number. `internal/query.Terms` splits the expression with the parser's own quoting
rules and reports each term's canonical key, so `p:`, `s:`, `a:` and `@` chips read as `project:`,
`status:` and `assignee:`.

## Typeahead from data the page has

The page already lists the tenant's projects, tags, workflows and actor handles for the add forms.
The toolbar writes them once into `<script type="application/json" id="filter-suggest">`; no new
endpoint and no request per keystroke. The decisions (which token the caret is in, what matches,
what the box becomes) are pure functions in decide.js; filterbar.js only draws the listbox. No option
is active until an arrow is pressed, so Enter keeps meaning Apply.

## Direction

`order=desc` sets the page's direction. The keyset builder already orders both ways, and the cursor
carries its direction, so a cursor from the other direction is refused as before; the form never
carries a cursor. Saved filters keep the sort field only, as before.

## `@handle`

`@raj` is `assignee:raj` and `-@raj` is `-assignee:raj`, in the shared parser, so the CLI, the TUI and
the API read the placeholder's example the same way. A quoted `"@raj"` stays free text.

## Measuring

`scripts/screenshots/shoot.mjs` reports the toolbar's height on every task screen it captures and,
under `--strict`, fails a desktop toolbar with no active filter taller than an eighth of the window, or
a phone toolbar of more than two rows.
