# Tasks

## 1. Layout primitives

- [x] 1.1 `internal/tui/layout.go`: `Wrap` and `WrapTitle`, word-aware, breaking a word longer than the
      line at its edge and truncating only the last line shown
- [x] 1.2 `internal/tui/layout.go`: `CardWindow` and `cardsFitting`, a window counted in lines rather
      than rows, keeping the selected card whole
- [x] 1.3 `internal/tui/layout.go`: `DistributeWidth`, `evenWidths` and `spreadRemainder`, sharing a
      width by demand with a per-column floor
- [x] 1.4 `internal/tui/wrap_test.go`: boundary tables for all four, including the exact fit, the cell
      either side of it, and the degenerate inputs
- [x] 1.5 The ten existing boundary tables in `layout_test.go` are untouched: `LayoutFor`, `VisibleRange`,
      `WindowLines`, `ScrollOffset`, `VisibleRows`, `Truncate`, `ScrollWindow`, `MoreBelow`, `ScrollHint`
      and `InnerWidth`/`CardRows` all keep their contracts

## 2. The card

- [x] 2.1 `internal/tui/card.go`: `Card`, `CardOf`, `CardMeta`, `CardHeights`, `CardBar` and the two bar
      glyphs
- [x] 2.2 `internal/tui/card.go`: `ColumnDemand` and `ColumnFloor`
- [x] 2.3 `internal/tui/view.go`: `cardLines` replaces `cardLine`, drawing the bar, the wrapped title and
      the dim identifier line
- [x] 2.4 `internal/tui/card_test.go`: the wrap, the line cost, the separated priority, selection without
      colour, the weighted width and the short empty column
- [x] 2.5 `internal/tui/motion_test.go`: the pulse guard follows `cardLines` and reads the selected bar

## 3. The board

- [x] 3.1 `internal/tui/view.go`: `columnWidths` weights the visible columns and `columnBlock` takes its
      own width
- [x] 3.2 `internal/tui/view.go`: `cardBlock` packs cards into a line budget with a blank line between
      them and the scroll hint below
- [x] 3.3 `internal/tui/view.go`: a column is drawn to the height of its contents, and `Width`/`Height`
      are given the outside of the box so the board uses its whole width
- [x] 3.4 `internal/tui/view.go`: one cell of gutter between a column's border and a card's bar

## 4. The task detail and the footer

- [x] 4.1 `internal/tui/view.go`: `bodyLines` and `commentLines` wrap to the pane's width instead of
      clipping at the right edge
- [x] 4.2 `internal/tui/view.go`: the board's task and column count appears on the board only
- [x] 4.3 `internal/tui/detail_test.go`: both, each reading the section it names rather than the frame

## 5. One input idiom

- [x] 5.1 `internal/tui/prompt.go`: `PromptSpec.Field` and `PromptSpec.Title`, and `choiceKind.Field`
- [x] 5.2 `internal/tui/view.go`: `Panel`, `panel`, `promptPanel`, `choicePanel`, `confirmPanel` and
      `formPanel` replace the four separate renderings
- [x] 5.3 `internal/tui/view.go`: `footerLines` withdraws the view's bindings while a panel is open
- [x] 5.4 `internal/tui/view.go`: `quiet` dims the body behind a panel, and `inputOpen` reports one
- [x] 5.5 `internal/tui/view.go`: `Frame` takes the panel's extra lines out of the body so the legend is
      never pushed off the bottom of the terminal
- [x] 5.6 `internal/tui/model.go`: `styleInput` strips the input bubble's own colours and caret where the
      theme draws none
- [x] 5.7 `internal/tui/input_test.go`: the shared shape, the withdrawn footer, the colourless rule and
      the picker's single keystroke
- [x] 5.8 `internal/tui/confirm_test.go`: `confirmLine` reads the question row and `confirmKeysLine` the
      legend row, rather than one line doing both

## 6. Documentation

- [x] 6.1 `docs/tui.md`: the card, the priority digits, the weighted width and the short column
- [x] 6.2 `docs/tui.md`: the one input idiom and what an open panel does to the footer
- [x] 6.3 `docs/tui.md`: what a colourless run keeps and what it loses
