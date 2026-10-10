# Design

## One binding or two

`EditTitle` was already doing two unrelated jobs. On the board it opened a title prompt; on the project
screen `handleSetupKey` matched the same binding and opened the project form, and on the tenant screen
`handleAdminKey` matched it and opened the tenant form. The name described the board's job only, and the
project screen had to carry its own description string (`"edit project"`) precisely because the binding's
own help text was wrong there.

So the shape was already one binding dispatched per view. What was wrong was the name and the board's
half of the dispatch. The merge is therefore a rename plus a deletion:

- `EditTitle` becomes `Edit`, help text `e` / `edit task`.
- `EditTask` is deleted. Its one call site, `openTaskEditForm`, moves onto `Edit`.
- `handleSetupKey` and `handleAdminKey` change one field name and nothing else.

Keeping two bindings was the alternative. It was rejected because the second one had no job left: the
form holds the title, so a title-only key on a task view is a second, slower route to part of what `e`
does. Two keys are worth having when they do different things, and after this change they would not.

`viewAction.desc` stays. A screen that borrows a board key still needs to describe what the key does
there, and "edit task" on a screen holding no task describes the board.

## promptTitle

Removed. After the merge its only opener was gone, and `runPrompt` and `promptSeed` were the only other
code that mentioned it. `promptNewTask` is a separate kind with its own spec and its own key (`n`) and is
untouched; it never went through `promptTitle`.

The removal is asserted rather than assumed: `TestNoKeyOpensATitleOnlyPrompt` walks `promptSpecs` and
fails if a prompt titled `title` is registered again.

## Which way the cycle turns, and whether it wraps

`p` steps **down** the scale: `P1` to `P2`, `P2` to `P3`, and so on. The priorities are numbered, and a
cycle over a numbered list conventionally counts up.

It **wraps** at `P5`, back to `P1`. Stopping at the bottom would leave a reader who overshot unable to
climb back with the key they overshot on, which makes the key a one-way ratchet rather than a cycle;
wrapping makes all five reachable from `p` alone, which is the point.

There is **no reverse key**. The shifted `p` is `P`, which is the picker and is worth more there than a
second direction would be: with wrapping, four presses reach anywhere the reverse could, and the picker
reaches it in one. A third priority key for the other direction would cost a letter and save a press.

It applies **immediately**. A cycle that opened something to confirm would be the picker with extra
steps.

## Where it applies

`CyclePriority` goes in `taskBindings`, which is the one list the footer, the help overlay and
`mayPress` all read. That puts it on the board and on the detail view together, which is the invariant
the repo already holds: every action that acts on the selected task is offered wherever a task is
selected, so the two views cannot drift apart. The detail view has the same selection and shows the same
priority, so a key that worked on the board and did nothing one level in would be the drift the shared
list exists to prevent.

It is gated on `UpdateTask`, the operation it calls, so a reader who may not edit is not offered it and
is refused at the keystroke rather than by the service. A deleted card is offered the restore and none of
the editing keys, and the cycle is an editing key, so it is correctly absent there.

## The collision on `p`, and what moved

`p` was `Projects`. `Projects` is a global: `handleKey` matches the cross-view keys before it reaches the
view's own handler, so a board-level `p` would never have been dispatched, and `Validate()` would have
refused the map anyway because `viewActions` lists the globals inside every view.

Something had to move. The alternatives were:

1. Put the cycle on another key. Rejected: the request names `p`, and `p` beside `P` for the same
   attribute is the pairing the rest of the map already uses (`v`/`V`-style capitals, `#`/`U`, `c`/`x`).
2. Drop `Projects` from the board's action list so `p` means the cycle there. Rejected: it would silently
   stop working on the one view it is used from most, and the key that discards the whole navigation
   stack is worth more than the collision check being appeased.
3. Move `Projects`. Chosen.

`W` is the capital of `w`, the project screen's key, so both keys about projects share a letter: `w` is
the open project's setup, `W` is every project. The house rule the map already states, that a taken
lowercase letter is replaced by its capital, could not be applied directly because `P` is the picker, so
neither case of `p` was free; borrowing the adjacent concept's letter is the nearest thing to it. `W` is
bound by no scheme, in any view.

## The five schemes

| Scheme | `Edit` | `CyclePriority` | `Priority` | `Projects` |
| --- | --- | --- | --- | --- |
| default | `e` | `p` | `P` | `W` |
| vim | `i` | `p` | `P` | `W` |
| emacs | `ctrl+t` | `p` | `P` | `W` |
| nano | `e` | `p` | `P` | `W` |
| helix | `i` | `p` | `P` | `W` |

`vim` and `helix` each bound `EditTask` to `I`; that line is deleted with the action. `vim` keeps `e` on
`Refresh`, which is why its `Edit` is `i` and not `e`, and that was already true. No scheme touches `p`,
`P` or `W`, so the cycle and the project list are the same keys everywhere.

`TestEveryShippedSchemeIsUsableAndCollisionFree` runs `Validate()` over all five in all nine views, which
is what proves there is no collision under `nano` or anywhere else rather than only under `default`.

## Documentation

`docs/tui.md` carries the bindings table, and `internal/tui/docs_test.go` asserts both directions: every
action's first key appears in it, and every key it names is bound to something. The row for `e`/`E` loses
`E`, a row for `p` is added beside `P`, and the cross-view row and the views table move to `W`. The
prose about editing a whole task and about `p` discarding the stack changes with them, and the `?`
overlay needs nothing because it is built from the `KeyMap`.
