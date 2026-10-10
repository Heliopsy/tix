# Tasks

## 1. One edit key

- [x] 1.1 `internal/tui/keys.go`: `EditTitle` becomes `Edit`, help `e` / `edit task`; `EditTask` is
      deleted, and `taskBindings`, `editHelp`, `projectBindings` and `tenantBindings` follow.
- [x] 1.2 `internal/tui/model.go`: `Edit` opens the whole-task form on a task view; `handleSetupKey`
      keeps the project form.
- [x] 1.3 `internal/tui/tenant.go`, `internal/tui/settings.go`: the renamed action.
- [x] 1.4 `internal/tui/scheme.go`: `Edit` in every scheme table and every `viewActions` list; the
      `EditTask` rows under `vim` and `helix` go.
- [x] 1.5 `internal/tui/prompt.go`, `model.go`: `promptTitle`, its spec, its `runPrompt` case and its
      seed are removed.

## 2. Cycling a priority

- [x] 2.1 `internal/tui/prompt.go`: `NextPriority`, one place down, wrapping at the bottom.
- [x] 2.2 `internal/tui/keys.go`: `CyclePriority` on `p`, in `taskBindings` against `UpdateTask` and in
      `editHelp` so the footer offers it.
- [x] 2.3 `internal/tui/keys.go`: `Projects` moves off `p` to `W`, which releases the key.
- [x] 2.4 `internal/tui/model.go`: `cyclePriority` sends the update at once, with nothing to answer.
- [x] 2.5 `internal/tui/scheme.go`: `CyclePriority` in the board's and the detail view's action lists.

## 3. Guards

- [x] 3.1 `internal/tui/cycle_test.go`: the step itself, table-driven over all five priorities including
      the wrap.
- [x] 3.2 `internal/tui/cycle_test.go`: `p` at the keyboard, on the board and in the detail view, with
      nothing left open to answer; the footer offers it; `P` still picks in one trip.
- [x] 3.3 `internal/tui/cycle_test.go`: `e` opens the whole-task form, and no title-only prompt is
      registered.
- [x] 3.4 `internal/tui/restore_test.go`: a deleted card's footer offers neither the edit nor the cycle.

## 4. Documentation

- [x] 4.1 `docs/tui.md`: the bindings table, the views table, the scheme table, the prose about the
      project-list key, the whole-task form and the new cycle section.
