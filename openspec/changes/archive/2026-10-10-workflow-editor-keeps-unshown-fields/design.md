# Design

## Where the merge belongs

In `internal/web`, in `putWorkflow`, before the service is called.

`PutWorkflow` replaces the stored definition with the one it is handed. That is
correct for the API and the CLI: both send a complete definition, and a caller
that omits `requires_scope` means to remove it. Moving the merge into the service
would make it impossible for any caller to clear a field, and it would put a rule
about one screen's form in the layer that is supposed to be surface-agnostic.

The partial form is the editor's problem, so the editor completes it. The service
still receives a complete definition and still replaces wholesale.

## What the merge does

The form edits four things: the initial state, the set of state keys with their
labels and terminality, the set of edges, and the migration lines. Everything
else is read back from the stored definition.

For each state the form names, the stored state under the same key is taken
whole, then `Key`, `Label` and `Terminal` are overwritten from the form. Taking
the stored value whole rather than copying named fields is what makes a field
added to `core.State` tomorrow survive without anybody remembering to extend the
merge.

Transitions work the same way, keyed on `(From, To)`. `DefaultLease` is copied
from the stored definition; the form has no control for it.

## Renaming a key

A merge keyed on the state key has nothing to match when the key changes, so the
old key's `Category` and lease-expiry revert would be silently defaulted away --
the same defect in a smaller box.

The form already has somewhere to say a new key is an old one: the migration
lines, `oldstate=newstate`, which exist because tasks sitting in the old state
have to be moved. The merge reads them as the rename they are, and follows
`RevertTo` through them as well.

An undeclared rename is not distinguishable from removing one state and adding
another, and is treated as exactly that. It is not silent:

- If any task sits in the removed state, `PutWorkflow` refuses the save and says
  a migration is required. The admin then writes the migration and the fields
  come across.
- If nothing sits in it, the save succeeds and the new state starts with
  defaults. That is the correct reading of a form that removed one state and
  added another, and the help text says so.

Two old keys migrating onto one new key leave no single predecessor, so that key
carries nothing over rather than picking one arbitrarily.

## Guarding it

A field added to `core.State` or `core.Transition` that the editor loses would be
this defect returning. `TestTheWorkflowEditorLosesNoFieldOfAStateOrATransition`
stores a state and a transition with every field set, uses reflection to fail if
any field is at its zero value, saves without editing anything and compares the
structs back field by field. A new field arrives zero-valued, so the guard stops
the build until somebody gives it a value and proves it round-trips.

The scope guard asserts the gate rather than the stored string:
`TestSavingTheWorkflowEditorUnchangedLeavesAGatedTransitionEnforced` attempts the
restricted move as an actor without the scope and requires a refusal, before and
after the save, and requires the same move to succeed for an actor who does hold
the scope, so the refusal cannot be a workflow the save broke.
