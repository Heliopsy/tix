# One edit key, and a priority you can cycle

## Why

Two reports, both about the board.

"make e the default not E we dont need specific edit for title when we are on the task"

`e` opened a one-field prompt for the title and `E` opened the whole-task form, which already holds the
title. Two keys, one of which does part of what the other does, and the smaller one has the letter a
reader reaches for first. The title-only prompt was a slower way to change the title than the form it
sits beside.

"also add p when in the kanban view to shift between priorities"

Changing a priority meant `P`, reading five numbered options, and pressing a digit. That is the right
trip when the reader knows which priority they want. Nudging a task one place down the scale, which is
what most priority edits are, should not need a menu.

## What Changes

- **`EditTitle` and `EditTask` merge into one `Edit` action on `e`.** It dispatches per view: the
  whole-task form on the board and the detail view, the project form on the project screen, the tenant
  form on the tenant screen. `E` is unbound.
- **The title-only prompt is removed.** Nothing opened it once `e` opened the form, so `promptTitle` and
  its spec go with it rather than staying as a prompt no key reaches.
- **`p` cycles the selected task's priority.** One press sends the next priority down the scale and
  applies it immediately, with nothing to pick. It wraps from `P5` back to `P1`, so every priority is
  reachable from the one key.
- **`P` keeps its picker.** Cycling is the fast path; naming a priority outright stays one trip rather
  than up to four presses.
- **The project list moves from `p` to `W`.** `p` was the projects key in every view, and a global key is
  matched before the board's own. Neither case of `p` was free afterwards, so the project list takes the
  capital of `w`, the project screen's key.
- **Every scheme follows.** `vim` and `helix` bind `Edit` to `i` and no longer bind `I`; `emacs` binds it
  to `ctrl+t`. No scheme moves `p`, `P` or `W`, and none of the five collides on them.

## Impact

- Affected specs: `tui-input`
- Affected code: `internal/tui/keys.go`, `internal/tui/scheme.go`, `internal/tui/model.go`,
  `internal/tui/prompt.go`, `internal/tui/tenant.go`, `internal/tui/settings.go`, `docs/tui.md`
- No service, store or capability change. Both keys call `UpdateTask`, which they already called.
- Two bindings move for existing readers: `E` no longer opens the form, and `p` no longer opens the
  project list. `docs/tui.md` and the `?` overlay say so, the overlay because it is built from the
  `KeyMap` itself.
