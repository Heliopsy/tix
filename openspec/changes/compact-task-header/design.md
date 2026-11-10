# Design

## The header line

The workflow, the summary and the field definitions are siblings in one flex line, separated by a
middle dot drawn in CSS, so the summary can stay the `#tasks-summary` block a live refresh replaces.
The workflow name is the link to the editor (titled "Workflow editor"), which drops one link and the
words around it. The task list keeps its summary as before, beside or under its heading.

## One row for quick add and the toolbar

The row is a size container, so the split follows the content column rather than the window: the
sidebar can be collapsed or dragged wider. At 84rem the row is a grid of `1fr` and `1.4fr`, and the
toolbar is told not to wrap, its box giving up width instead. That is a Full HD window with the
sidebar open; a 1440px laptop stacks as before. A narrower threshold would leave the quick-add box a
few characters wide once Bulgarian labels are in the toolbar.

A single "+ New task" control that expands was the other option. It would hide the one-line box,
which is the fastest way in and which the `n` shortcut focuses, behind a click.

## The card's Move button

Hiding the button at rest uses `opacity`, not `display` or `visibility`, so it stays in the tab
order, and `:focus-within` on the card shows it when it is reached. The rule applies only under
`(hover: hover) and (pointer: fine)`; a touch screen always draws it, at 44px. It stays a `<details>`
holding the Move form, the one fold `TestNoPageActionOrSectionSitsInAnExpander` allows on a card, so
drag and drop, which reads that form's select, and the scriptless post both keep working.

## The board's workflow line

On a project page the board is always that project's, and the header names its workflow. On the task
list a board of one project had a line saying "1 project sharing it". The line now appears only on a
merged board, where it says something the cards do not.
