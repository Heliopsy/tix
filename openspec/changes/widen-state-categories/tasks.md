# Tasks

## 1. The vocabulary, in core

- [x] 1.1 `internal/core/types.go`: `CategoryBlocked`, `CategoryWaiting` and `CategoryCancelled`;
      `stateCategories` as the ordered vocabulary, `StateCategories()` returning a copy, `Valid()` and
      `JoinStateCategories()`, modelled on `ProjectColor`
- [x] 1.2 `internal/core/input.go`: `WorkflowInput.Validate` refuses a state naming a word outside the
      vocabulary, naming the state, the word and all six accepted categories. Nothing validated
      categories before this change
- [x] 1.3 `internal/core/input_test.go`: the refusal and its accepted-set message, each of the six
      accepted, the empty category accepted, the returned vocabulary not being the vocabulary itself,
      and `cancelled` not collapsing onto `done`

## 2. The shipped workflow

- [x] 2.1 `internal/service/bootstrap.go`: `blocked` becomes `CategoryBlocked`, `cancelled` becomes
      `CategoryCancelled`
- [x] 2.2 `internal/sshd/seed.go`: the same two states, so the seeded workflow does not drift from the
      shipped one
- [x] 2.3 Stored workflows are deliberately not migrated; `EnsureDefaults` stays idempotent and rewrites
      nothing

## 3. Statistics

- [x] 3.1 `internal/service/stats.go`: `statsCategoryOrder` becomes the vocabulary rather than a
      three-element literal, so a category added to core cannot be counted into nothing
- [x] 3.2 `statsCategoryOf` keeps deriving a category for a state that names none; unchanged
- [x] 3.3 `internal/service/stats_test.go`: cancelled work counted apart from done, blocked apart from
      todo, every category reported when empty, a hand-authored workflow keeping its declared category,
      terminal-ness unaffected, and a cancelled task still carrying a completion timestamp

## 4. Colour

- [x] 4.1 `internal/output/color.go`: `styleWaiting` and `styleCancelled`; `StatusIn` answers all six;
      `knownCategories` maps `blocked`, `waiting`, `on_hold`, `on-hold`, `cancelled` and `canceled` onto
      the new categories
- [x] 4.2 `internal/tui/theme.go`: `colorWaiting`; `CategoryColor` answers all six, with `blocked`
      borrowing the urgent colour and `cancelled` the muted one
- [x] 4.3 `internal/web/render.go`: labels for the three new categories, and `categoryClass` giving each
      one a css class
- [x] 4.4 `internal/web/templates/stats.html`: the breakdown's category cell carries that class
- [x] 4.5 `internal/web/assets/app.css`: one rule per category, keyed off the category rather than off
      the state key, plus `--cat-waiting` declared once so every theme scope resolves its own violet
- [x] 4.6 `internal/output/color_test.go`: the six painted distinctly, degrading with colour off, an
      unknown category painted plainly, and the parameter each category paints pinned against the board
- [x] 4.7 `internal/tui/category_test.go`: the six drawn distinctly, the column heading and the card bar
      both drawn by the theme's category style, cancelled not drawn like done, and every category
      writing no attribute on a colourless terminal
- [x] 4.8 `internal/web/category_test.go` and `lease_internal_test.go`: the breakdown row named and
      classed per category, no two categories resolving to one colour in the stylesheet,
      `--cat-waiting` actually declared, and an unknown category carrying no class

## 5. Documentation

- [x] 5.1 `docs/workflows.md`: a categories table, the fixed-vocabulary rationale, what changes and what
      does not when `cancelled` stops being `done`, and the example workflow's dropped state
- [x] 5.2 `docs/statistics.md`: that `completed` and the breakdown answer different questions about a
      cancelled task, and that the breakdown reports six rows
- [x] 5.3 `docs/tui.md`: the six categories and which two exist to stop a misreading

## 6. Gates

- [x] 6.1 Every guard watched to fail against a mutation of the code it protects, restored, `git diff`
      confirmed empty
- [x] 6.2 `just check`
- [x] 6.3 `just test-postgres`
- [x] 6.4 Hand verification against a scratch database: a board with states in all six categories, the
      statistics screen, and the same board on a colourless terminal
