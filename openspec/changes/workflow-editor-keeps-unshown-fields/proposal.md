# The workflow editor keeps the fields it does not show

## Why

A tenant admin who opened `/workflows/default`, changed nothing and pressed Save
lost every part of the workflow the editor does not render.

The editor writes a state as `key|label|terminal` and a transition as `from>to`.
`core.State` also carries `Category`, `RevertOnLeaseExpiry` and `RevertTo`.
`core.Transition` also carries `RequiresScope` and `RequiresComment`.
`WorkflowDefinition` also carries `DefaultLease`. None of them were rendered, so
the parsed form came back missing all of them, and `PutWorkflow` replaces the
stored definition wholesale.

One of those fields is an authorization gate. `Transition.RequiresScope` is read
in `internal/service/task.go` and `internal/service/claim.go`, and an empty value
means the edge is open to anybody holding the plain `task:transition` scope. A
save that changed nothing quietly relaxed a restricted move for everybody. The
rest is not cosmetic either: the stats screen groups by `Category`, and losing
`RevertOnLeaseExpiry`/`RevertTo` on `doing` stops an abandoned task returning to
`todo` when its lease expires.

The obvious fix -- widen the editor's line format to carry the missing fields --
is a UX decision, and it would lose the next field added to `core.State` the same
way. The defect is not the format. It is that a partial form replaces a whole
definition.

There is a second, smaller report in the same file. The parser accepted only the
literal word `terminal` in the third field, while the help text under the textarea
told the reader to write `true` or `false`. `done|Done|true` was accepted without
complaint and stored `Terminal: false`: data loss for following the instructions.

## What Changes

- The workflow editor merges its form onto the stored definition instead of
  replacing it. A state or transition the form still names keeps every field the
  form does not edit; one the form dropped is dropped.
- A state key the form renamed carries its unshown fields across when the save
  declares the rename in the migration lines, which is where the form already
  says an old key became a new one.
- The state line's third field accepts `true`/`false` as well as
  `terminal`/`open` and their usual synonyms, and refuses a word that is neither
  rather than reading it as "open".
- The help text says what the parser accepts and says that unshown fields are
  kept.
- `PutWorkflow` is unchanged. Replace-wholesale is the right contract for the API
  and the CLI, which send a complete definition.
