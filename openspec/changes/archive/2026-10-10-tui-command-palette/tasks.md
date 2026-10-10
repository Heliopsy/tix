# Tasks

## 1. One action list

- [x] 1.1 `internal/tui/keys.go`: `actionID` and the constant for every action the interface performs
- [x] 1.2 `internal/tui/keys.go`: `Action` replaces `globalKey`, carrying the id, the binding, the view it
      opens, the gate, and whether it needs a selected task or an open project
- [x] 1.3 `internal/tui/keys.go`: `globalKeys` and `taskBindings` return `Action`, and `Actions` is the two
      of them in one list
- [x] 1.4 `internal/tui/keys.go`: `GlobalHelp` and `taskActions` filter that list rather than their own
- [x] 1.5 `internal/tui/keys.go`: `Palette` added to `KeyMap`, `DefaultKeyMap` and `globalKeys`, bound to
      `ctrl+k` and `:`, hidden from the palette's own listing

## 2. Dispatch through the list

- [x] 2.1 `internal/tui/model.go`: `performAction`, one switch, every body moved from `handleKey` and
      `handleTaskKey` unchanged
- [x] 2.2 `internal/tui/keys.go`: `Resolve` resolves a key press to an action
- [x] 2.3 `internal/tui/model.go`: `handleKey` resolves the global actions through `Resolve` instead of ten
      hand-written cases
- [x] 2.4 `internal/tui/model.go`: `handleTaskKey` resolves through the task actions and asks the gate on
      the way through; `mayPress` removed as the second copy of that question
- [x] 2.5 `internal/tui/palette_test.go`: every action is performed, and every action resolves from its own
      first key

## 3. The palette

- [x] 3.1 `internal/tui/palette.go`: `PaletteEntry`, `PaletteContext`, `PaletteActions` and `Palette`
- [x] 3.2 `internal/tui/palette.go`: `MatchAction`, pure, word-wise and case-insensitive
- [x] 3.3 `internal/tui/palette.go`: `openPalette`, `closePalette`, `handlePaletteKey`, `runPaletteEntry`
      and `movesWithin`
- [x] 3.4 `internal/tui/model.go`: the palette is a mode `handleKey` routes to, beside the prompt and the
      picker
- [x] 3.5 `internal/tui/view.go`: `palettePanel` renders through the existing `panel`, with the scroll hint
      and the no-match line
- [x] 3.6 `internal/tui/palette_test.go`: the matcher table, the authority filter, the selection filter,
      the current-view marker, the empty query, the no-match line and the key-versus-entry equality

## 4. The footer hint

- [x] 4.1 `internal/tui/palette.go`: `FooterHint`, empty below the width its own text needs
- [x] 4.2 `internal/tui/view.go`: `footerLines` puts the hint first so truncation eats the view's keys
- [x] 4.3 `internal/tui/palette_test.go`: the hint is in every view's footer, survives `MinWidth`, and is
      dropped whole below the width it needs

## 5. Schemes

- [x] 5.1 `internal/tui/scheme.go`: `Palette` added to the global action list so collisions are checked
- [x] 5.2 `internal/tui/scheme.go`: `vim` and `helix` keep `:` for the filter and take `ctrl+k`; `nano`
      keeps `ctrl+k` for delete and takes `:`
- [x] 5.3 `internal/tui/scheme.go`: the scheme descriptions name the palette key they move

## 6. Documentation

- [x] 6.1 `docs/tui.md`: the palette section, with the transcript, the matching rule and the order
- [x] 6.2 `docs/tui.md`: `ctrl+k` in the default bindings table, and the palette key in the scheme table
- [x] 6.3 `docs/tui.md`: the footer hint and what a narrow terminal does to it
