# Design

## The mode lives on the toggle

Select mode is one attribute: `aria-pressed` on the toolbar's Select toggle (`[data-select-toggle]`).
The server draws it pressed when the address carries `select=1`; the script flips it. The stylesheet
shows the row boxes and the bulk bar only under `.wrap:has([data-select-toggle][aria-pressed="true"])`.

The boxes are always in the markup and hidden by CSS outside the mode, rather than drawn only for
`select=1`. That is what makes the mode survive every partial redraw for free: a live refresh, a
row's own swap after a tick, the in-place filter and an htmx swap all redraw rows that already carry
their box, and none of them touches the toggle, which sits in the page header outside every swapped
region. A boosted navigation replaces the header too, so leaving the screen leaves the mode, and the
address decides again. Drawing the boxes only for `select=1` would have meant a script building
boxes into rows that arrive later, with their labels translated on the client, and losing them on
every morph.

Without CSS (a text browser) the boxes show all the time, which is the old behaviour.

## Selection survives swaps

The script keeps the selected references in a set held on the toggle element, and writes it back to
the boxes after every `htmx:load`. A row ticked done in select mode comes back from the server with
an unchecked box; the set puts the tick back. A navigation brings a new toggle and so an empty set.
The live refresh already waits while a box is ticked (`regionBusy`), so this matters for the row
swap and for the in-place filter.

## Without scripting

The toggle is a link: `?select=1` when off, the same address without it when on. The Cancel button in
the bulk bar is the same "off" link. The boxes belong to the bar's form through `form="bulkbar"`, as
before, so ticking and pressing an action posts the selection. The bulk route returns to `next`,
which is the screen's own address, so the reader lands back in select mode.

`select` is kept by the view switch and the lanes control (they keep every parameter but a few), by
the pager (added to its kept list), by the filter form (a hidden field), by Clear (kept beside the
sort and the view) and by Show all. It is dropped by nothing else, because nothing else rebuilds the
address.

## Entering and leaving

- Toggle: flips the mode. Turning it off clears the selection.
- `x` on a focused row: enters the mode if needed and toggles the row.
- Shift-click on a row's empty area: enters the mode and selects from the anchor to the row. Ctrl or
  Cmd-click toggles the row. A click on a link, button, field, label or menu is not an empty area, so
  Ctrl-click on a title still opens a tab. In the mode, a plain click on an empty area toggles too.
- Long press on touch: 500ms with the finger within 10px, not on a button or field. The click the
  release would produce is swallowed. Scrolling or moving cancels it.
- Esc (when no menu, dialog, overlay or field has it), Cancel, or unselecting the last selected task
  leaves the mode. Entering with the toggle and selecting nothing stays in the mode until Cancel or Esc.

## Keys

`x` keeps the existing `select` action. Two actions are added, `extendUp` and `extendDown`, on
`shift+ArrowUp` and `shift+ArrowDown`; `comboOf` now prefixes `shift+` to named keys (one character
keys already carry their case). Shift+Enter, Shift+Escape and Shift+Home no longer reach `open`,
`back` and `top`, which is what one would expect of them anyway.

The done key was asked for as `e`. `e` is Edit title in the default and emacs schemes, the guard
`TestKeyBindingsForHaveNoCollision` refuses a key bound twice, and `TestDefaultSchemeMirrorsTheTUIDefaults`
pins `e` to the terminal's edit key. Done takes `d` in every scheme instead: free everywhere, and the
word it starts. It presses the row's round control, and does nothing on a task that is already done
or on a board card, which has no round control.

## Cancel, not Done

The bar's way out was asked for as Done or Cancel. The catalog is keyed by the English text and
"Done" is already the workflow state, translated as a task's state ("Готова"), so a Done button
would read as a status in Bulgarian. Cancel ("Отказ") is already a button everywhere else.

## Board and lanes

Board cards and lane cards carry the same box, first in the card, joined to the same bar. The bar is
drawn for every view with a bulk action to offer, not only the list. A card being dragged is not
affected: the box is a form control, and dragging starts from the card's body.

## Visual

The done control keeps its round shape and is dimmed to 45% in the mode (full on hover and focus).
The selection box is square with a 3px radius, the accent colour when ticked, and shown only in the
mode. A selected row or card takes the accent tint.
