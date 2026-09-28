# Design

## The card

Three shapes were on the table. A denser single line, a two-column grid, and a title-led card with the
identifiers dim underneath. The third was chosen, for two reasons.

The first is that a title is what a person scans a board for. The reference is how they address a task
afterwards, to the CLI or to a colleague, and it does not need to be the first thing on the line. Leading
with the title gives it the column's whole width, which is what stops the wrapping from happening at all
on most cards.

The second is convergence with the browser. `internal/web/templates/board.html` draws an article per task
with a monospace reference, the title as the link, and a meta row beneath. The two surfaces were drifting
into looking like different products. The terminal now carries the same three parts.

Where it deliberately diverges: the web puts the reference *above* the title and the terminal puts it
below. On the web the reference is small, monospace and grey, so it reads as a label over the heading; in
a terminal every cell is the same size and a leading reference simply delays the title, which is the
defect being fixed. The terminal also drops the tag chips the web card carries, because a tag list is
unbounded and a column is twenty cells wide at its floor.

## Selection without colour

The constraint is absolute: a frame drawn without colour carries no escape sequence anywhere, so
selection cannot be a colour, a weight or a reverse video attribute. It is the shape of the bar. `┃` for
selected, `│` for not, one cell each so nothing shifts between them. Both are box-drawing characters the
interface already leans on for its column borders, so a font that renders one renders the other.

The pulse is unchanged. It varies the style the selected bar and title are rendered in, and
`MotionEnabled` already refuses to run without colour, so a colourless terminal gets a static heavy bar
rather than invisible motion on a busy wire.

## Variable card heights

Cards are no longer all the same height, so a window counted in rows would either clip a card in half or
leave a card's worth of blank frame under the last one. `CardWindow` counts a line budget instead: it
takes the heights of the column's cards and returns the first card visible and how many fit, pulling the
window so the selected card is whole. `Layout.CardRows` and `VisibleRows` are untouched and keep their
existing boundary guards; the model still keeps a card-index hint in `rowOff` and `CardWindow` corrects it
at render time.

## Weighted width

`LayoutFor` still decides how many columns fit and at what uniform width, so the ten boundary tables in
`layout_test.go` still describe it exactly. `DistributeWidth` is a separate function layered on top: each
column is given the smaller of its demand and its floor, and the spare is shared in proportion to the room
each still wants. A column's demand is its heading or its widest title, whichever is larger, capped at
`MaxCardWidth` so one long title cannot take the board. A column holding work floors at `MinColumnWidth`;
an empty one may shrink to its heading, because every cell it gives up goes to a column that has something
to show.

The leftover from integer division goes to the columns that asked for more room, not to the ones already
holding everything they have.

## One input idiom

The delete form was already the right shape, so everything converges on it rather than on a new
invention. `Panel` is title, note, rows and a legend; each of the four modes builds one. A prompt becomes
a one-field form by giving every `PromptSpec` the name of the answer it gathers, and the field's label
sits at the same column a form's labels sit at.

The footer stops drawing the view's bindings while a panel is open. This is the part that was actively
wrong rather than merely plain: a legend offering `n new task` under a text field describes a keystroke
that types the letter n.

Focus is shown three ways, so that losing colour loses only one of them: a rule the width of the terminal
above the panel, the board behind it drawn through the dim style, and the legend naming this mode's keys
rather than the view's. The rule and the legend survive `NO_COLOR`.

### The cost of that

The `textinput` bubble draws its caret in reverse video and its text in its own colours, neither of which
the theme controls. Under a colourless run both are now suppressed, which keeps the documented promise
that such a frame carries no escape at all, and costs the caret. The panel's rule and legend are what say
where the typing is going. The alternative was to let one widget break the invariant the rest of the
interface keeps, and the invariant is load-bearing: it is what `tix ssh` into a `dumb` terminal relies on.

## What was deliberately left alone

The statistics view, the help overlay and the task detail's label/value block work. The detail's body
clipping and the wrong footer were defects that happened to live there and are fixed; nothing else about
those three screens was redesigned.
