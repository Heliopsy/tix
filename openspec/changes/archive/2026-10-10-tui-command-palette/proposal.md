# One key that names every action

## Why

`internal/tui/keys.go` holds forty-nine bindings. Every lowercase letter is taken except `a b d i o s z`,
and the file says so out loud: "the obvious letters are spoken for", "W, not p", "S, not s: lowercase s is
taken". The capitals are being handed out by elimination rather than by meaning. `W` lists projects and `w`
opens project setup. `H` reads history and `v` draws the live tail. `L` picks a tag, `O` records an
artifact, `N` claims the next task. Nobody holds that in their head, and the next feature has seven letters
left.

The complaint that started this was that there is no easy way to open the activity tail or the settings
screen. The keys existed. Nothing advertised them, and `?` only helps a reader who already knows that `?`
is the key.

A go-to menu for moving between views was the first idea. It is a strict subset of the answer: one key
opens a searchable list of every action, each named in words and each showing its own key, so the surface
is discoverable without memorising anything and the pressure comes off every future binding.

## What Changes

- **One key opens a palette of every action.** `ctrl+k`, and `:` where no scheme has already spent it.
  Substring matching over the action's name, `enter` runs the highlighted one, `esc` cancels, the arrows
  move.
- **One list, three readers.** The bindings, the human names and what each action needs live in one table.
  The `?` overlay renders from it, the palette renders from it, and a key press resolves through it into
  the one switch that performs the action. A palette entry cannot do something slightly different from its
  key, because it is the key's own code path.
- **Nothing is offered that cannot be done.** An entry is filtered by the same `TUIAccess` predicate the
  help overlay already filters with, and an action on the selected task is not listed while nothing is
  selected. The open view is marked on its own entry.
- **Each entry shows its key**, so the palette teaches the bindings rather than replacing them.
- **The footer names the key, permanently.** It is placed first, so a narrow terminal drops the view's own
  bindings before it drops the one that leads to all of them, and below the width the hint needs it is
  dropped whole rather than cut into a key nobody can press.
- **No separate go-to menu.** The palette covers it.

## Impact

- Affected specs: `tui-palette`
- Affected code: `internal/tui/palette.go` (new), `internal/tui/keys.go`, `internal/tui/model.go`,
  `internal/tui/view.go`, `internal/tui/scheme.go`, `docs/tui.md`
- No service, store or capability change. The palette performs the actions the interface already had, so
  no new `core.Service` method and no new `capability.Registry` entry: the operation count and the
  terminal gap count both stand still.
