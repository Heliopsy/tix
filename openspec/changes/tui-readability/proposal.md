# A board you can read, and one way to answer a question

## Why

The terminal board is the surface people spend their day in, and at a hundred and fifty columns with five
workflow states it is the hardest thing in the product to read.

A card is one line: a reference, a priority badge glued to its markers, then whatever is left of the
title. When the title does not fit, and it usually does not, it wraps to an unindented fragment at column
zero and is truncated a second time, so two lines are spent showing twelve characters of title. Every
card costs those two lines whether or not the second carries anything, and a column of eight cards is
sixteen lines of `pl…`, `to…`, `nod…`.

The width is split equally, so `Cancelled (0)`, whose entire content is the word `empty`, is given the
same twenty-five columns as `Done (8)`. The busy columns starve so the idle ones can hold nothing. Empty
columns are then drawn to the full height of the terminal, so most of the board is blank frame.

Away from the board, the task detail clips its body at the right edge rather than wrapping it, so a
sentence ends `…write the allow rules from o` with nothing saying it was cut, and the footer reports `14
tasks in 5 columns` under a view showing one task.

Input has the same problem from the other end. There are three idioms for "give me an answer" and only
one of them is any good. A bare prompt is a single line under the board with no structure and no
statement of what `enter` or `esc` do. A numbered picker is a third shape again. Only the delete form has
field and value in columns and its own key legend. Worse, all three leave the board's footer underneath,
still advertising `n new task` while pressing `n` types the letter n: the interface spends every input
telling the reader to do something that cannot work. Nothing dims or otherwise says the keyboard has been
taken.

## What Changes

- **The card leads with the title.** Title first at the column's full width, identifiers dim beneath, and
  a bar down the left edge carrying the workflow category as colour and the selection as a heavier glyph.
  A title too long for one line wraps onto a second indented to the same column, and only the last line
  shown is cut. A card whose title fits spends no line saying nothing.
- **Selection stops depending on colour.** The heavier bar is the cue, so `NO_COLOR`, `TIX_NO_COLOR`, a
  `dumb` terminal and a colourless theme all still say which card the keys will act on. The pulse keeps
  varying the selected card's emphasis where there is colour to vary.
- **Width is shared by what a column has to show**, floored at enough for its own heading, and a column
  is drawn as tall as what it holds rather than as tall as the terminal.
- **The priority is separated from the markers**, so `P2*` stops reading as one token, and the page says
  what `P1` through `P5` mean.
- **The task detail wraps its body** instead of clipping it, and the board's column count appears only on
  the board.
- **One idiom for input.** Prompt, picker, confirmation and form all render as the same panel: a rule the
  width of the terminal, what is being asked, one labelled row per answer at the form's own column, and
  the keys that end it. A single-value prompt is a form with one field.
- **An open panel owns the footer and dims the board.** The view's bindings are not advertised while they
  would be typed as text. The rule and the panel's own legend carry that without colour.
- **The picker keeps its single keystroke.** `t` still costs one digit; only its framing changes.

## Impact

- Affected specs: `tui-board`, `tui-input`
- Affected code: `internal/tui/layout.go`, `internal/tui/card.go` (new), `internal/tui/view.go`,
  `internal/tui/prompt.go`, `internal/tui/model.go`, `docs/tui.md`
- No service, store or capability change. Every view keeps the operations it had.
