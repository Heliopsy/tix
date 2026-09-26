# Design

## Why the project screen is one view rather than three

Projects, workflows and field definitions are three registry families and one subject. A reader asking "why
does this board have a review column" and a reader asking "why does this task demand a severity" are looking
at the same configuration from two sides, and a terminal that made them two screens would have made the
answer two navigations.

One view also keeps the gating honest. A view is offered when the reader may perform one of the reads bound to
it, so the screen is offered on `project.show` or `workflow.get`, and a reader refused workflows still reads
the project with a line saying why the state machine is not shown.

## Why `workflow.put` gets no binding

A workflow is a state machine: states with a category and a terminal flag, the edges between them, what each
edge requires of whoever takes it, a default lease, and a migration for the tasks already sitting in a state
that is going away. The terminal has two inputs, a single line of free text and a form whose every field picks
from a fixed list, and neither can gather a graph. A form over an arbitrary state machine would be a worse
editor than the file `tix workflow put` already takes, and offering a worse one would move the work into a
place that cannot finish it. So the screen renders the definition and names the command that changes it.

## Why `workflow.delete` gets no binding either

The only workflow the terminal names is the one the open project runs on, and the service refuses to delete a
workflow a project is still assigned to. The single place a terminal could offer the key is the single place
it would always be refused, which is the shape the interface already rejects elsewhere: an entry leading to a
refusal is worse than no entry. Deleting a workflow needs a listing of every workflow and the projects using
each, which is what `tix workflow ls` is.

## Why `project.show` is a binding and not an exemption

It would have been bookkeeping a day ago, when nothing in the terminal showed one project. The project screen
shows exactly one, and it shows attributes it then edits and archives. `ArchiveProject` returns nothing, so
after an archive the only way to know what the project now is, is to read it; relisting every project to learn
the state of one is the wrong shape, and the row the screen was opened from is stale. The fetch is the read
the screen renders from, which is the test of whether a binding implements an operation.

## Why the field definition is gathered in two steps

A form cannot reseed the rows below an answer when that answer changes, so a single form holding "which field"
above "what type" would show the first field's type no matter which field was picked. The field is therefore
chosen in one form and its shape gathered in a second, seeded from the definition that was actually picked.

Enum is offered only to a field that already is one. An enum needs its options, which no list of fixed
alternatives can gather; a field that already has them keeps them, and a new field cannot be made an enum
here. A definition narrowed off enum drops the options with it, because the input refuses a string field
carrying them.

## Why the keys are the board's own

The screen borrows `EditTitle`, `Delete` and `New` rather than adding three bindings. Every scheme then drives
it with no new entries in any scheme table: a reader who moved the edit key onto `i` edits a project with `i`.
What the screen does not borrow is the description, because "edit title" on a screen holding no task describes
the board. Only the two keys that had no equivalent are new: `w`, which opens the screen, and `f`, which
offers the field definitions.
