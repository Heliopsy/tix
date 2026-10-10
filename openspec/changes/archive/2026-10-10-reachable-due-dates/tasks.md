# Tasks

## 1. One classification, in core

- [x] 1.1 `internal/core/due.go`: `DueState`, `DueStateOf`, `DueSoonWindow`, `Notable`, `String` and
      `Task.DueState`, standard library only
- [x] 1.2 `internal/core/due_test.go`: the boundaries — a deadline exactly at now, the last instant of the
      window and the cell past it — and the notability rule every surface draws by
- [x] 1.3 `internal/query/match.go`: make both bounds inclusive, matching `due_at <= ?` and `due_at >= ?`
      in the stores, so a page the store returned is not then dropped on screen

## 2. The filter language

- [x] 2.1 `internal/query/query.go`: split `Parse` into `Parse` and `ParseAt(expr, now)`, so a term about
      the present takes the present as an input
- [x] 2.2 `internal/query/query.go`: the `due:` term — `overdue`/`late`, `today`, `week`, `month`, and the
      bounded `due:<DATE` and `due:>DATE`; `DueWindows` as the one list of the named windows
- [x] 2.3 `internal/query/query.go`: list `due` in `Keys` and among the terms that cannot be negated
- [x] 2.4 `internal/query/due_test.go`: each window's bound, both bounded forms, the refusals, and the
      inclusive boundary against `Matches`

## 3. The command line

- [x] 3.1 `cmd/task.go`: `--overdue`, `--due-before` and `--due-after` on `task ls`, parsed with the
      `parseTime` `--due` already uses
- [x] 3.2 `cmd/task.go`: `applyDueFlags` — the flag beats the expression's bound, `--overdue` beside
      `--due-before` is a usage error, and the pair is validated
- [x] 3.3 `cmd/task.go`: the long help and the examples name the flags and their expression spelling
- [x] 3.4 `cmd/due_test.go`: the three flags and the two expression forms against one seeded backlog, plus
      the refusals and the flag-beats-expression precedence

## 4. The board

- [x] 4.1 `internal/tui/theme.go`: `DueColor` and `Theme.Due`, built the way `PriorityColor` and
      `Theme.Priority` are, plain when the theme carries no colour
- [x] 4.2 `internal/tui/card.go`: `MetaSegment`, `CardMetaSegments`, `DueMarker` and `TruncateSegments`,
      cutting the whole line once and handing the result back to its runs
- [x] 4.3 `internal/tui/view.go`: `metaLine` renders each run in its own style; `CardLegend` gains both
      markers; the legend is wrapped rather than cut
- [x] 4.4 `internal/tui/view.go`: `dueText` names the state beside the date on the detail, and `timeText`
      gives the deadline up
- [x] 4.5 `internal/tui/due_test.go`: what is marked and what is not, the marker drawn by the theme rather
      than by a literal, the colourless terminal, the three-deadline board, the detail, and the cell
      budget at every column width down to the floor
- [x] 4.6 `docs/tui.md`: both markers in the table the legend is asserted against, and why only two states
      are drawn

## 5. The browser

- [x] 5.1 `internal/web/render.go`: the `due` template function, reading the one classification
- [x] 5.2 `internal/web/tasks.go`: the `due` parameter folded into the expression before parsing, carried
      by the pager, and reported back to the control
- [x] 5.3 `internal/web/columns.go`: `Due` as an optional column of the task listing, on by default
- [x] 5.4 `internal/web/templates/tasks.html`: the control beside the sort, and the row badge
- [x] 5.5 `internal/web/templates/task.html`: the badge beside the date in the details rail
- [x] 5.6 `internal/web/assets/app.css`: `.duemark`, outlined like a lease, danger and warn
- [x] 5.7 `internal/web/dueui_test.go`: the badge rule per row, the control narrowing the listing, the
      control agreeing with the typed expression, the refusal, and the task screen

## 6. The parity guard

- [x] 6.1 `internal/capability/filterparity_test.go`: the reach table and the two exemption tables, each
      entry carrying its reason
- [x] 6.2 The structural check over `reflect.TypeOf(core.TaskFilter{})`, and the check that every recorded
      term really sets its field
- [x] 6.3 The check that the terminal and the browser answer an expression the way the shared parser does
- [x] 6.4 The CLI check, read off the real `tix task ls` flag list rather than off `--filter`
- [x] 6.5 The HTTP round trip: every field through the real client and the real router, compared with what
      the recording service was handed

## 7. Documentation

- [x] 7.1 `docs/filtering.md`: the `due:` rows, the deadlines section, and `due` among the terms that
      cannot be negated
- [x] 7.2 `docs/api.md`: what `due_before` and `due_after` mean, and why there is no `overdue` parameter
- [x] 7.3 `docs/scripting.md`: the overdue recipe
- [x] 7.4 `docs/web-ui.md`: the deadline control and the row badge
- [x] 7.5 `skills/tix/SKILL.md`: the flags and the term, with the refusals a caller has to expect
