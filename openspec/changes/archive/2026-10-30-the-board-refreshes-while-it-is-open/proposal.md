# The board refreshes while it is open

## Why

The project board at `/projects/{key}` never live-updates. Opening it and
moving a task from anywhere else, the terminal, another tab, an agent, leaves
the board on its first paint for as long as it stays open. A reload shows the
change, so the data is right and only the screen is stale. Present in v0.16.0
and v0.17.0, on both SQLite and PostgreSQL. The task list and the activity
feed update correctly.

The socket is connected and the events arrive. The refresh is deferred, every
time, forever:

Each card's Move control renders its routes with no option marked selected.
A browser presented with such a `<select>` selects the first option anyway, so
the DOM reports `option.selected === true` against `option.defaultSelected ===
false`. That is exactly the shape of a selection the reader made.

`tix.fieldDirty` in `assets/decide.js` compares those two fields and nothing
else, so it calls every card on the board dirty. `tix.regionBusy` therefore
reports `#project-board` as permanently mid-edit, `busyScreen` in
`assets/live.js` agrees, and `refresh()` takes the `defer()` branch on every
pass of the 500ms re-arm. The fetch of `data-live` is never made.

The rule the deferral implements is right: a refresh that lands on a
half-written comment or a held drag destroys work the reader cannot get back.
The board is simply the only screen with a `<select>` inside a live region,
which is why it is the only one that froze, and why nothing else showed the
bug.

## What Changes

**The Move control declares its own default.** `board.html` marks the first
route option `selected`, so what the browser presents matches what the markup
asked for and the DOM reports no change.

**`fieldDirty` stops inventing a choice nobody made.** Where no option in a
`<select>` declares itself the default, the control is read as untouched when
it sits on the first option and dirty when it sits on any other, rather than
dirty on sight. A `<select>` that does declare a default is still read against
that default, unchanged.

Both are needed. The first repairs the board as it ships today. The second
means the next template that forgets a `selected` attribute renders a slightly
wrong menu instead of silently freezing a live region.

## Impact

- `internal/web/templates/board.html`: the Move control's first option.
- `internal/web/assets/decide.js`: the `SELECT` branch of `fieldDirty`.
- `internal/web/boardlive_test.go` (new), `internal/web/jstest/decide.test.mjs`,
  `internal/web/jstest/live.test.mjs`.

**What does not change.** The deferral rule itself, the subscription filters,
the fragment routes, and every other screen's behaviour. No server-side
behaviour changes at all: the stream, the handlers and the stores are
untouched, which is why both storage engines showed the same board.
