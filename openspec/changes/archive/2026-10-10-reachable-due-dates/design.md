# Design

## One classification, in core

`core.DueState` answers "how does this deadline stand against now" once: `DueNone`, `DueLater`, `DueSoon`,
`DueOverdue`. Every surface that draws a deadline reads it, so the board, the browser listing, the browser
detail and the filter shorthand cannot come to disagree about what "overdue" means.

It lives in `internal/core` rather than in `internal/output` because the browser needs it too and does not
import the CLI's rendering package, and because it is a property of a task rather than of a rendering.
It imports only `time`, so the frozen contract stays standard-library only.

Two boundaries are chosen rather than inherited:

- A deadline **exactly at now** is overdue. The stores filter with `due_at <= ?`, so a task at the bound is
  returned by a listing; it has to read as overdue wherever it is drawn, or a listing and its rows would
  disagree.
- **`DueSoonWindow` is seven days.** It has to be long enough to be worth saying and short enough not to
  cover the backlog: the marker this replaces was drawn for any due date at all, which put it on nearly
  every card. A week is the horizon a backlog is planned over, and the browser control offers the same
  word.

Fixing the boundary in core also fixed a divergence found on the way: `query.Matches` compared the due
bounds *exclusively* while both stores compare them inclusively, so a task falling due at the exact
instant a bound named was returned by the store and then dropped by the board filtering the page it had
just been handed. `matchDue` now matches the SQL.

## `due:` is a spelling, not a capability

`due:` sets `DueBefore` or `DueAfter` and nothing else. No new field on `core.TaskFilter`, no new store
predicate, no new wire parameter, no new capability registry entry.

The named windows are relative to a present, so `query.Parse` is split: `ParseAt(expr, now)` takes the
moment as an input and `Parse(expr)` is `ParseAt(expr, time.Now().UTC())`. A parser reaching for the wall
clock is untestable; a parser that cannot reach one at all forces every caller to thread a clock through
for the sake of one term. Both are available, and the tests use `ParseAt`.

`due:` is **not negatable**, for the reason `due-before:`, `sort:` and `limit:` are not: it names a shape
of the listing rather than a set of tasks, and "not falling due before Tuesday" is not a question the
stores can answer against a column that may be null.

The alternative considered was `overdue:true` as an `is:` state — `is:overdue`. It was rejected because
`is:` states are answered by a column the store already has, and this one would be a bound computed at
parse time wearing a predicate's clothes; a reader could reasonably expect `-is:overdue` to work, and it
cannot.

## The card marker

The meta line was one string rendered in one style. A marker whose colour carries meaning cannot live in
one style, and the line is truncated to the column's text width, so the obvious fix — style a suffix after
truncating — breaks the moment the cut lands inside the marker.

`Card.Meta` is therefore `[]MetaSegment`, each segment a run of text and the due state it is drawn in.
`TruncateSegments` cuts the *whole* line once with the existing `Truncate` and hands the result back to the
segments it came from, so:

- the line drawn is exactly the line `Truncate` would have produced, ellipsis included, which is what keeps
  the width guarantee the cell-measuring work established;
- a run the cut lands inside keeps its own style, so half a marker is still drawn as a marker.

The marker sits after the priority and **before** the other markers, because it is the one of them that
changes by itself and so the one that has to survive a narrow column.

Colour comes from `Theme.Due(state)`, built the same way `Theme.Priority` and `Theme.Category` are: a
`DueColor` function naming the colour and the weight, a method that returns a plain style when the theme
carries no colour, and `Foreground` so the colour is flattened to the terminal's declared depth. Overdue
borrows `colorUrgent`, which "blocked" already uses, because both say the same thing to somebody scanning
a board and a sixth hue on a card would be a distinction without a difference. `due` borrows `colorTodo`.

The markers are words, so a colourless terminal keeps the meaning and loses only the emphasis, which is
the rule the selection bar and the status pills already follow.

The help legend is now **wrapped** rather than drawn on one line. It was one line on the promise that its
entries would stay short enough to fit, which a narrow terminal has never kept; two more entries made that
plain rather than creating it.

## The browser control

The deadline control submits `?due=WINDOW`, and the handler folds it into the filter *expression* as
`due:WINDOW` before parsing, rather than setting a bound on the parsed filter. Three things follow: a bad
window is refused by the one parser with the one message under the box, the reader can see and correct
what was asked, and the control cannot drift from the language. The term leads the expression, so a `due:`
the reader typed by hand wins.

The badge is a `Due` column in the existing column picker, on by default, so a reader who does not want it
can turn it off the way they turn off `Updated`.

## The parity guard

`capability.Registry` checks that an operation is *bound* on each surface. Nothing checked that the bound
surfaces accept the same inputs, which is the hole this whole change walked through.

`internal/capability/filterparity_test.go` reflects over `core.TaskFilter` and requires every field to be
either reachable or exempted by name with a written reason — the same shape as `capability.Exemption`,
because an absence somebody decided on reads differently from one nobody noticed. Reach is checked, not
declared:

- **Expression**: the recorded term is parsed and the field asserted non-zero, so a table of typos fails.
- **Terminal and browser**: the same expression through `tui.ParseFilter` and `web.ParseFilter` must give
  the same filter, so a surface that grew its own parser again fails.
- **CLI**: the recorded flag must appear in the real `tix task ls` help. `--filter` reaches every field by
  construction, so a guard written against it would pass whatever the flag list held; the flag list is what
  is read. A field with no flag needs a reason in `cliFlagExempt`.
- **HTTP**: a filter with every field set goes through the real `client.Client` to the real
  `httpapi.Router`, and what the service is handed is compared with what was sent, field by field. It names
  no parameters, so a field the client does not encode or the handler does not read fails whatever either
  of them is spelled.

This was worth its cost. It is roughly three hundred lines and it reproduces the reported defect on
demand: deleting `--overdue` from `task ls` fails it by name, and so does deleting `due_before` from the
client's query builder.

The guard deliberately does **not** cover `core.Page`. Sort, limit, cursor and direction are the shape of
a listing rather than a set of tasks, each already has its own control on each surface, and folding them
in would make the table about pagination rather than about filtering.
