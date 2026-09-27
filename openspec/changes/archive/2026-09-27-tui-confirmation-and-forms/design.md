# Design

## The confirmation is a value, not a dialogue

`Confirm` holds the kind of action, the target it will affect and a note for the reach the target cannot
convey, plus the identifiers the released call needs. `Question()` is a pure function of the first three.

The target is load-bearing rather than decorative. `Question()` returns the empty string when the target is
empty, and `openConfirm` refuses to open a confirmation whose question is empty, so there is no code path that
puts an unnamed destructive question on screen. The guard is asserted against `Question()` itself and against
the one rendered line the confirmation draws, located by the instruction only it renders, so neither assertion
can be satisfied by the ref the title bar and the card already print.

`y` answers it. `enter` deliberately does not: `Accept` is the key that submits every prompt and every form
in the interface, and a confirmation answered by that key is answered by muscle memory.

## The form is the settings screen's mechanic, generalised

The settings screen already moved a cursor between rows and cycled each row through the values it accepts.
The form reuses that vocabulary, including `CycleValue`, and is driven by `Up`, `Down`, `Left`, `Right`,
`Accept` and `Cancel`. That is what makes it work under every scheme for free: the schemes rebind those keys,
and the form asks the loaded `KeyMap` rather than hardcoding any of them. Its own help line prints the keys
the loaded scheme bound, so a nano reader is told `ctrl+o apply`.

There is no free-text field kind. None of the five operations wanted one: a delete chooses among its own
switches, and picking a tag or a dependency chooses among the ones that exist. Free text already has the
single-line prompt. `artifact.put` will want a text field, and inventing one now would have been a guess about
an operation nobody was building.

`FieldCondition` hides a field until another field's answer makes it relevant, and a hidden field answers
nothing through `Value`, so a caller cannot act on a switch the reader was never shown.

## Building the five is what fixed the shapes

- The confirmation started as a yes-or-no question and became **form then confirmation**, because
  `DeleteTaskInput` carries `Hard` and `Cascade`. A confirmation is the wrong place to gather two switches and
  the right place to state their consequence, which is why the question carries a note and the form does not
  decide anything on its own.
- The subject of a deletion became a **form field** rather than a second destructive key. `task.delete` and
  `comment.delete` arrived together, the interface has one key left that reads as destruction, and the
  alternative was a key whose meaning depended on what was selected.
- Selecting a dependency started as the existing numbered picker and became a **form field**, because the
  picker reads a single digit. A task can wait on more than nine others, and the alternative was recording a
  limitation against an operation that had just stopped being a gap.
- The form and the selector turned out to be one primitive. A choice field over the values that exist is what
  "pick one of these" means, and that is why `tag.list` and `dependency.remove` needed no mechanism of their
  own.

## Selecting a comment

Comments are the only list the interface draws inside another view's body, so a second scrolling cursor was
not wanted. The detail view listens for `Left` and `Right`, which mean columns on the board and meant nothing
here, and relabels them in the footer the way `settingHelp` already relabels them for the settings screen.
The selected comment carries `SelectionMarker`, the marker a selected card carries.

The cursor is only advertised when the open task has a thread, and it is only offered in the detail view, so
the board's column keys keep their own meaning.

## Authority is asked of the registry, once

`ViewAccess` is supplied by the caller because the set of reachable views is a session fact a caller resolves
from `capability.TUIAccess`. `ActionAccess` is resolved here instead, from `capability.Operations()` and the
actor the model already holds, because it is a pure function of those two and there is nothing for a caller to
forget. It is still not a second opinion: the scope each operation needs is recorded once, beside the
operation, and this reads that record rather than restating it.

`KeyMap.taskBindings` pairs every action on the selected task with the operation it calls. The footer, the
help overlay and `handleTaskKey` all filter through it, so the three cannot disagree about who may press a
key. The delete key names two operations and is offered when either is permitted, which is also how the delete
form decides which subjects to offer.

## Trade-offs

- `#` and `U` keep taking a typed name. `L` is a third tag key rather than a replacement, because a reader who
  knows the name should not have to walk a list, and a reader who does not had no list to walk.
- Gating every task action, not only the new ones, changes what a restricted reader sees in the footer. That
  is the intended behaviour, and it is why the test board now names a full-authority actor: an action missing
  from the frame should mean the interface left it out, not that the test's reader was refused it.
